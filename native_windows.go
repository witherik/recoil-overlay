//go:build windows && amd64

package main

import (
	"context"
	_ "embed"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	wr "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

var user32 = windows.NewLazySystemDLL("user32.dll")
var kernel32 = windows.NewLazySystemDLL("kernel32.dll")
var createWindow = user32.NewProc("CreateWindowExW")
var destroyWindow = user32.NewProc("DestroyWindow")
var registerRaw = user32.NewProc("RegisterRawInputDevices")
var rawData = user32.NewProc("GetRawInputData")
var getMessage = user32.NewProc("GetMessageW")
var dispatchMessage = user32.NewProc("DispatchMessageW")
var postThreadMessage = user32.NewProc("PostThreadMessageW")
var foregroundWindow = user32.NewProc("GetForegroundWindow")
var windowText = user32.NewProc("GetWindowTextW")
var windowPID = user32.NewProc("GetWindowThreadProcessId")
var enumWindows = user32.NewProc("EnumWindows")
var getWindowLong = user32.NewProc("GetWindowLongPtrW")
var setWindowLong = user32.NewProc("SetWindowLongPtrW")
var setWindowPos = user32.NewProc("SetWindowPos")
var playSound = windows.NewLazySystemDLL("winmm.dll").NewProc("PlaySoundW")

// Every clip remains rooted for the entire process lifetime (async PlaySound).
//
//go:embed assets/voice/left.wav
var voiceLeft []byte

//go:embed assets/voice/right.wav
var voiceRight []byte

//go:embed assets/voice/natural/left.wav
var naturalLeft []byte

//go:embed assets/voice/natural/right.wav
var naturalRight []byte

// The tones are synthesized: a low beep for left, an octave higher for right.
var toneLeft, toneRight = toneClip(440), toneClip(880)

// toneClip is a 60 ms sine beep as a mono PCM16 WAV, faded so it does not click.
func toneClip(hz float64) []byte {
	const rate, samples, fade = 22050, 22050 * 60 / 1000, 110
	clip := make([]byte, 44+samples*2)
	copy(clip, "RIFF")
	binary.LittleEndian.PutUint32(clip[4:], uint32(len(clip)-8))
	copy(clip[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(clip[16:], 16)
	binary.LittleEndian.PutUint16(clip[20:], 1) // PCM
	binary.LittleEndian.PutUint16(clip[22:], 1) // mono
	binary.LittleEndian.PutUint32(clip[24:], rate)
	binary.LittleEndian.PutUint32(clip[28:], rate*2)
	binary.LittleEndian.PutUint16(clip[32:], 2)
	binary.LittleEndian.PutUint16(clip[34:], 16)
	copy(clip[36:], "data")
	binary.LittleEndian.PutUint32(clip[40:], samples*2)
	for i := 0; i < samples; i++ {
		gain := math.Min(1, math.Min(float64(i+1), float64(samples-i))/fade)
		value := 0.6 * 32767 * gain * math.Sin(2*math.Pi*hz*float64(i)/rate)
		binary.LittleEndian.PutUint16(clip[44+2*i:], uint16(int16(value)))
	}
	return clip
}

func playVoice(style, direction string) {
	left, right := voiceLeft, voiceRight
	switch style {
	case "natural":
		left, right = naturalLeft, naturalRight
	case "tones":
		left, right = toneLeft, toneRight
	}
	clip := right
	if direction == "left" {
		clip = left
	}
	playSound.Call(uintptr(unsafe.Pointer(&clip[0])), 0, 0x0001|0x0002|0x0004) // ASYNC | NODEFAULT | MEMORY
	runtime.KeepAlive(clip)
}
func stopVoice() { playSound.Call(0, 0, 0) }

type inputEvent struct {
	Kind   string
	At     time.Time
	Apex   bool
	Detail string
	X, Y   int
	Code   int // keyboard scan code for "key" events
	Down   bool
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
	n, _, _ := user32.NewProc("GetKeyNameTextW").Call(lparam, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)))
	if n == 0 {
		return fmt.Sprintf("Key %#x", code)
	}
	return windows.UTF16ToString(text[:])
}

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
type winMessage struct {
	Window         uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	X, Y           int32
	Private        uint32
}

func startNativeInput(events chan<- inputEvent) (*nativeInput, error) {
	n := &nativeInput{done: make(chan struct{})}
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(n.done)
		id, _, _ := kernel32.NewProc("GetCurrentThreadId").Call()
		n.threadID = uint32(id)
		cls, _ := windows.UTF16PtrFromString("STATIC")
		hwnd, _, err := createWindow.Call(0, uintptr(unsafe.Pointer(cls)), 0, 0, 0, 0, 0, 0, ^uintptr(2), 0, 0, 0) // HWND_MESSAGE = -3
		if hwnd == 0 {
			ready <- fmt.Errorf("create input window: %w", err)
			return
		}
		defer destroyWindow.Call(hwnd)
		// Background mouse and keyboard, read-only: nothing is suppressed.
		devices := [2]rawDevice{{Page: 1, Usage: 2, Flags: 0x100, Target: hwnd}, {Page: 1, Usage: 6, Flags: 0x100, Target: hwnd}}
		ok, _, err := registerRaw.Call(uintptr(unsafe.Pointer(&devices[0])), 2, unsafe.Sizeof(devices[0]))
		if ok == 0 {
			ready <- fmt.Errorf("register mouse and keyboard input: %w", err)
			return
		}
		defer func() {
			for i := range devices {
				devices[i].Flags = 1
				devices[i].Target = 0
			}
			registerRaw.Call(uintptr(unsafe.Pointer(&devices[0])), 2, unsafe.Sizeof(devices[0]))
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
			if msg.Message == 0x00FF {
				at := time.Now()
				// Fixed aligned buffer covers RAWINPUTHEADER (24 bytes on x64) plus
				// RAWMOUSE (24) or RAWKEYBOARD (16).
				var buffer [16]uint64
				size := uint32(unsafe.Sizeof(buffer))
				count, _, _ := rawData.Call(msg.LParam, 0x10000003, uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)), 24)
				if count >= 40 && count <= unsafe.Sizeof(buffer) && size <= uint32(unsafe.Sizeof(buffer)) {
					bytes := unsafe.Slice((*byte)(unsafe.Pointer(&buffer[0])), int(size))
					if binary.LittleEndian.Uint32(bytes[:4]) == 1 {
						code := uint32(binary.LittleEndian.Uint16(bytes[24:26]))
						flags := binary.LittleEndian.Uint16(bytes[26:28])
						if flags&2 != 0 {
							code |= 0xE000 // extended key (arrows, right-hand modifiers)
						}
						down := flags&1 == 0
						if watched(code) || (down && captureKey.Load()) {
							events <- inputEvent{Kind: "key", At: at, Code: int(code), Down: down}
						}
					}
					if count >= 48 && binary.LittleEndian.Uint32(bytes[:4]) == 0 {
						flags := binary.LittleEndian.Uint16(bytes[28:30])
						if flags&1 != 0 {
							events <- inputEvent{Kind: "down", At: at, Apex: apexForeground()}
						}
						if flags&2 != 0 {
							events <- inputEvent{Kind: "up", At: at}
						}
					}
				}
			}
			dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}()
	if err := <-ready; err != nil {
		return nil, err
	}
	return n, nil
}
func (n *nativeInput) Close() {
	n.once.Do(func() {
		postThreadMessage.Call(uintptr(n.threadID), 0x0012, 0, 0)
		select {
		case <-n.done:
		case <-time.After(time.Second):
		}
	})
}

func titleOf(hwnd uintptr) string {
	var text [512]uint16
	windowText.Call(hwnd, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)))
	return windows.UTF16ToString(text[:])
}
func apexForeground() bool {
	hwnd, _, _ := foregroundWindow.Call()
	title := titleOf(hwnd)
	return title == "Apex Legends" || title == "Apex Legendsâ„¢"
}
func ownWindow() uintptr {
	var found uintptr
	callback := windows.NewCallback(func(hwnd, extra uintptr) uintptr {
		var pid uint32
		windowPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid == uint32(os.Getpid()) && strings.EqualFold(titleOf(hwnd), windowTitle) {
			found = hwnd
			return 0
		}
		return 1
	})
	enumWindows.Call(callback, 0)
	return found
}

// The Wails window is created with WS_EX_NOREDIRECTIONBITMAP (transparent
// WebView), which can neither be removed nor combined with WS_EX_LAYERED. So
// the arrows live in a separate native layered window with its own position.
var mainWindow, practiceWindow uintptr
var practiceRect struct{ Left, Top, Right, Bottom int32 }
var practiceBase point // top-left of the overlay when its offset is zero
var practiceDPI uintptr
var practicePlaced bool

const (
	overlayHidden = iota
	overlayPractice
	overlayPreview // as in practice, but beside the settings window
	overlayMove
)

// Move mode: the overlay accepts the mouse and follows it while the left
// button is held. Tracked by hand; the system's caption drag does not start on
// this caption-less layered window.
var overlayMovable atomic.Bool
var overlayMoved func(offsetX, offsetY int)
var overlayDragging bool
var overlayGrab point // cursor position inside the window at button-down
var overlayProc = windows.NewCallback(func(hwnd, msg, wparam, lparam uintptr) uintptr {
	if overlayMovable.Load() || overlayDragging {
		var cursor point
		var rect struct{ Left, Top, Right, Bottom int32 }
		switch msg {
		case 0x0084: // WM_NCHITTEST
			return 1 // HTCLIENT, so button messages arrive here
		case 0x0021: // WM_MOUSEACTIVATE
			return 3 // MA_NOACTIVATE
		case 0x0201: // WM_LBUTTONDOWN
			user32.NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&cursor)))
			user32.NewProc("GetWindowRect").Call(hwnd, uintptr(unsafe.Pointer(&rect)))
			overlayGrab = point{cursor.X - rect.Left, cursor.Y - rect.Top}
			overlayDragging = true
			user32.NewProc("SetCapture").Call(hwnd)
			return 0
		case 0x0200: // WM_MOUSEMOVE
			if overlayDragging {
				user32.NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&cursor)))
				setWindowPos.Call(hwnd, 0, uintptr(cursor.X-overlayGrab.X), uintptr(cursor.Y-overlayGrab.Y), 0, 0, 0x0001|0x0004|0x0010)
				return 0
			}
		case 0x0202, 0x0215: // WM_LBUTTONUP, WM_CAPTURECHANGED
			if overlayDragging {
				overlayDragging = false
				if msg == 0x0202 {
					user32.NewProc("ReleaseCapture").Call()
				}
				user32.NewProc("GetWindowRect").Call(hwnd, uintptr(unsafe.Pointer(&rect)))
				if overlayMoved != nil {
					overlayMoved(int(rect.Left-practiceBase.X), int(rect.Top-practiceBase.Y))
				}
				return 0
			}
		}
	}
	result, _, _ := user32.NewProc("DefWindowProcW").Call(hwnd, msg, wparam, lparam)
	return result
})

// Must be called on a thread that keeps pumping messages (or owns the window).
func createPracticeWindow() (uintptr, error) {
	cls, _ := windows.UTF16PtrFromString("STATIC")
	// layered | transparent | noactivate | toolwindow | topmost; WS_POPUP
	hwnd, _, err := createWindow.Call(0x00080000|0x20|0x08000000|0x80|0x8, uintptr(unsafe.Pointer(cls)), 0, 0x80000000, 0, 0, 0, 0, 0, 0, 0, 0)
	if hwnd == 0 {
		return 0, fmt.Errorf("create practice overlay: %w", err)
	}
	setWindowLong.Call(hwnd, ^uintptr(3), overlayProc) // GWLP_WNDPROC
	return hwnd, nil
}

// The overlay gets its own thread so dragging it (a modal loop inside the
// window procedure) never stalls the Raw Input listener.
func startOverlayWindow() error {
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		hwnd, err := createPracticeWindow()
		if err != nil {
			ready <- err
			return
		}
		practiceWindow = hwnd
		ready <- nil
		var msg winMessage
		for {
			result, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if int32(result) <= 0 {
				return
			}
			dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
		}
	}()
	return <-ready
}

// Screen position of the crosshair: the middle of Apex's client area, or of the
// monitor holding our window when the game isn't running.
func crosshairPoint() (int32, int32) {
	for _, name := range []string{"Apex Legends", "Apex Legendsâ„¢"} {
		title, _ := windows.UTF16PtrFromString(name)
		hwnd, _, _ := user32.NewProc("FindWindowW").Call(0, uintptr(unsafe.Pointer(title)))
		if minimized, _, _ := user32.NewProc("IsIconic").Call(hwnd); hwnd == 0 || minimized != 0 {
			continue
		}
		var client struct{ Left, Top, Right, Bottom int32 }
		user32.NewProc("GetClientRect").Call(hwnd, uintptr(unsafe.Pointer(&client)))
		middle := point{client.Right / 2, client.Bottom / 2}
		if ok, _, _ := user32.NewProc("ClientToScreen").Call(hwnd, uintptr(unsafe.Pointer(&middle))); ok != 0 && client.Right > 0 {
			return middle.X, middle.Y
		}
	}
	var info struct {
		Size          uint32
		Monitor, Work struct{ Left, Top, Right, Bottom int32 }
		Flags         uint32
	}
	info.Size = uint32(unsafe.Sizeof(info))
	monitor, _, _ := user32.NewProc("MonitorFromWindow").Call(mainWindow, 2) // nearest
	user32.NewProc("GetMonitorInfoW").Call(monitor, uintptr(unsafe.Pointer(&info)))
	return (info.Monitor.Left + info.Monitor.Right) / 2, (info.Monitor.Top + info.Monitor.Bottom) / 2
}

// Puts the midpoint between the arrows on the crosshair, shifted by the user's
// saved offset. An offset that would leave the desktop is ignored.
func placeOverlay(s Settings) {
	practiceDPI, _, _ = user32.NewProc("GetDpiForWindow").Call(mainWindow)
	if practiceDPI == 0 {
		practiceDPI = 96
	}
	scale := float64(practiceDPI) / 96
	width, height := overlaySize(s, scale)
	x, y := crosshairPoint()
	practiceBase = point{x - width/2, y - int32(math.Round(arrowAnchorY*scale))}
	left, top := practiceBase.X+int32(s.OffsetX), practiceBase.Y+int32(s.OffsetY)
	metric := user32.NewProc("GetSystemMetrics")
	screenLeft, _, _ := metric.Call(76)
	screenTop, _, _ := metric.Call(77)
	screenWidth, _, _ := metric.Call(78)
	screenHeight, _, _ := metric.Call(79)
	if left+width <= int32(screenLeft) || top+height <= int32(screenTop) || left >= int32(screenLeft)+int32(screenWidth) || top >= int32(screenTop)+int32(screenHeight) {
		left, top = practiceBase.X, practiceBase.Y
	}
	practiceRect.Left, practiceRect.Top, practiceRect.Right, practiceRect.Bottom = left, top, left+width, top+height
	practicePlaced, practiceFrameValid = false, false
}

func setOverlayMode(mode int, s Settings) error {
	if mainWindow == 0 {
		mainWindow = ownWindow()
		if mainWindow == 0 {
			return fmt.Errorf("overlay window not found")
		}
	}
	showWindow := user32.NewProc("ShowWindow")
	// Async for the Wails window: its UI thread may be waiting on our lock.
	showAsync := user32.NewProc("ShowWindowAsync")
	overlayMovable.Store(mode == overlayMove)
	if mode == overlayHidden {
		showWindow.Call(practiceWindow, 0)
		showAsync.Call(mainWindow, 5)
		return nil
	}
	if practiceWindow == 0 {
		return fmt.Errorf("practice overlay unavailable")
	}
	placeOverlay(s)
	extended, _, _ := getWindowLong.Call(practiceWindow, ^uintptr(19))
	if mode == overlayMove {
		extended &^= 0x20 | 0x08000000
	} else {
		extended |= 0x20 | 0x08000000 // click-through, never focused
	}
	setWindowLong.Call(practiceWindow, ^uintptr(19), extended)
	showWindow.Call(practiceWindow, 4) // SW_SHOWNOACTIVATE; invisible until the first frame
	// Above the settings panel too, so the overlay is never hidden behind it.
	setWindowPos.Call(practiceWindow, ^uintptr(0), 0, 0, 0, 0, 0x0001|0x0002|0x0010)
	switch mode {
	case overlayPractice:
		showAsync.Call(mainWindow, 0)
	case overlayPreview:
		showAsync.Call(mainWindow, 5)
	}
	return nil
}
func restorePosition(ctx context.Context, x, y int) {
	metric := user32.NewProc("GetSystemMetrics")
	left, _, _ := metric.Call(76)
	top, _, _ := metric.Call(77)
	width, _, _ := metric.Call(78)
	height, _, _ := metric.Call(79)
	if x < int(int32(left)) || y < int(int32(top)) || x > int(int32(left))+int(width)-100 || y > int(int32(top))+int(height)-100 {
		wr.WindowCenter(ctx)
		return
	}
	// Wails v2 SetPosition adds the current monitor's work-area origin; our saved
	// WindowGetPosition values are already absolute screen coordinates.
	hwnd := ownWindow()
	if hwnd != 0 {
		setWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), 0, 0, 0x0001|0x0004|0x0010)
	}
}
