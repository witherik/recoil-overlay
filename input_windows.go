//go:build windows && amd64

package main

import (
	"encoding/binary"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// An inputEvent is something the app must react to, sent by the native
// threads: Kind is "down" or "up" for the left mouse button, "key" for a
// watched key, "moved" when the overlay has been dragged, and "error" when the
// listener has died.
type inputEvent struct {
	Kind   string
	At     time.Time
	Apex   bool // "down": Apex was the foreground window
	Detail string
	X, Y   int  // "moved": the overlay's new offset
	Code   int  // "key": keyboard scan code; 0xE000 marks extended keys
	Down   bool // "key": pressed rather than released
}

// The listener forwards only the bound keys (two strafe keys, three hotkeys),
// or the next key pressed while a binding is being captured. Every other
// keystroke is dropped unread.
var watchedKeys [5]atomic.Uint32
var captureKey atomic.Bool

func watched(code uint32) bool {
	for i := range watchedKeys {
		if watchedKeys[i].Load() == code {
			return true
		}
	}
	return false
}

// keyName returns the label Windows prints on the key with this scan code.
func keyName(code int) string {
	lparam := uintptr(code&0xFF) << 16
	if code&0xE000 != 0 {
		lparam |= 1 << 24
	}
	var text [64]uint16
	n, _, _ := getKeyNameText.Call(lparam, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)))
	if n == 0 {
		return fmt.Sprintf("Key %#x", code)
	}
	return windows.UTF16ToString(text[:])
}

// nativeInput is the Raw Input listener: a thread with a message-only window.
type nativeInput struct {
	threadID uint32
	done     chan struct{}
	once     sync.Once
}

type rawDevice struct {
	Page, Usage uint16
	Flags       uint32
	Target      uintptr
}

// startNativeInput starts listening for the mouse and keyboard in the
// background. Input is only read: nothing is suppressed.
func startNativeInput(events chan<- inputEvent) (*nativeInput, error) {
	n := &nativeInput{done: make(chan struct{})}
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(n.done)
		n.threadID = windows.GetCurrentThreadId()
		cls, _ := windows.UTF16PtrFromString("STATIC")
		hwnd, _, err := createWindow.Call(0, uintptr(unsafe.Pointer(cls)), 0, 0, 0, 0, 0, 0, ^uintptr(2), 0, 0, 0) // HWND_MESSAGE = -3
		if hwnd == 0 {
			ready <- fmt.Errorf("create input window: %w", err)
			return
		}
		defer destroyWindow.Call(hwnd)
		const mouse, keyboard = 2, 6         // usages on the generic desktop page
		const inputSink, remove = 0x100, 0x1 // RIDEV_INPUTSINK (background input), RIDEV_REMOVE
		devices := [2]rawDevice{{Page: 1, Usage: mouse, Flags: inputSink, Target: hwnd}, {Page: 1, Usage: keyboard, Flags: inputSink, Target: hwnd}}
		register := func() error {
			ok, _, err := registerRaw.Call(uintptr(unsafe.Pointer(&devices[0])), uintptr(len(devices)), unsafe.Sizeof(devices[0]))
			if ok == 0 {
				return err
			}
			return nil
		}
		if err := register(); err != nil {
			ready <- fmt.Errorf("register mouse and keyboard input: %w", err)
			return
		}
		defer func() {
			for i := range devices {
				devices[i].Flags, devices[i].Target = remove, 0
			}
			register()
		}()
		ready <- nil
		var msg winMessage
		for {
			result, _, err := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if int32(result) == -1 {
				events <- inputEvent{Kind: "error", Detail: fmt.Sprintf("Input listener stopped: %v", err)}
				return
			}
			if result == 0 {
				return
			}
			if msg.Message == wmInput {
				readRawInput(msg.LParam, events)
			}
			dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}()
	if err := <-ready; err != nil {
		return nil, err
	}
	return n, nil
}

// readRawInput turns one WM_INPUT message into events: presses and releases
// of the left mouse button and of the watched keys.
func readRawInput(handle uintptr, events chan<- inputEvent) {
	at := time.Now()
	// Fixed aligned buffer covers RAWINPUTHEADER (24 bytes on x64) plus
	// RAWMOUSE (24) or RAWKEYBOARD (16).
	var buffer [16]uint64
	size := uint32(unsafe.Sizeof(buffer))
	count, _, _ := rawData.Call(handle, 0x10000003, uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)), 24) // RID_INPUT
	if count < 40 || count > unsafe.Sizeof(buffer) || size > uint32(unsafe.Sizeof(buffer)) {
		return
	}
	bytes := unsafe.Slice((*byte)(unsafe.Pointer(&buffer[0])), int(size))
	const mouse, keyboard = 0, 1 // RIM_TYPEMOUSE, RIM_TYPEKEYBOARD
	switch device := binary.LittleEndian.Uint32(bytes[:4]); {
	case device == keyboard:
		code := uint32(binary.LittleEndian.Uint16(bytes[24:26]))
		flags := binary.LittleEndian.Uint16(bytes[26:28])
		if flags&2 != 0 {
			code |= 0xE000 // extended key (arrows, right-hand modifiers)
		}
		down := flags&1 == 0
		if watched(code) || (down && captureKey.Load()) {
			events <- inputEvent{Kind: "key", At: at, Code: int(code), Down: down}
		}
	case device == mouse && count >= 48:
		flags := binary.LittleEndian.Uint16(bytes[28:30])
		if flags&1 != 0 {
			events <- inputEvent{Kind: "down", At: at, Apex: apexForeground()}
		}
		if flags&2 != 0 {
			events <- inputEvent{Kind: "up", At: at}
		}
	}
}

// Close stops the listener and waits a moment for its thread to end.
func (n *nativeInput) Close() {
	n.once.Do(func() {
		postThreadMessage.Call(uintptr(n.threadID), wmQuit, 0, 0)
		select {
		case <-n.done:
		case <-time.After(time.Second):
		}
	})
}
