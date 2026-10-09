//go:build windows && amd64

package main

import (
	"encoding/binary"
	"golang.org/x/sys/windows"
	"os"
	"runtime"
	"testing"
	"time"
	"unsafe"
)

func TestWindowsInputLayouts(t *testing.T) {
	if unsafe.Sizeof(rawDevice{}) != 16 || unsafe.Sizeof(winMessage{}) != 48 {
		t.Fatal("Win32 structure layout mismatch")
	}
}

func TestNativeLayeredWindowRoundTrip(t *testing.T) {
	if os.Getenv("RECOIL_NATIVE_TEST") != "1" {
		t.Skip("native opt-in")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	cls, _ := windows.UTF16PtrFromString("STATIC")
	title, _ := windows.UTF16PtrFromString(windowTitle)
	// Hidden test-owned window: no input injection or interaction with another app.
	hwnd, _, err := createWindow.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(title)), 0x00040000, 0, 0, 620, 600, 0, 0, 0, 0)
	if hwnd == 0 {
		t.Fatal(err)
	}
	defer destroyWindow.Call(hwnd)
	// Mirror the real Wails window, which cannot become layered.
	overlay, err := createPracticeWindow()
	if err != nil {
		t.Fatal(err)
	}
	defer destroyWindow.Call(overlay)
	mainWindow, practiceWindow = 0, overlay
	defer func() { mainWindow, practiceWindow, practiceFrameValid = 0, 0, false }()
	isVisible := user32.NewProc("IsWindowVisible")
	windowRect := func() (rect struct{ Left, Top, Right, Bottom int32 }) {
		user32.NewProc("GetWindowRect").Call(overlay, uintptr(unsafe.Pointer(&rect)))
		return
	}
	s := Snapshot{Settings: defaultSettings(), Armed: true, Direction: "right"}
	if err := setOverlayMode(overlayPractice, s.Settings); err != nil {
		t.Fatal(err)
	}
	if err := renderPractice(s); err != nil {
		t.Fatal(err)
	}
	style, _, _ := getWindowLong.Call(overlay, ^uintptr(19))
	if style&(0x80000|0x20|0x08000000) != (0x80000 | 0x20 | 0x08000000) {
		t.Fatal("missing layered/click-through/noactivate flags")
	}
	centred := windowRect()
	if visible, _, _ := isVisible.Call(overlay); visible == 0 || centred != practiceRect {
		t.Fatalf("practice overlay not shown: %+v, want %+v", centred, practiceRect)
	}
	x, y := crosshairPoint()
	if centred.Left+(centred.Right-centred.Left)/2 != x || centred.Top+int32(arrowAnchorY*float64(practiceDPI)/96+.5) != y {
		t.Fatalf("arrows not centred on crosshair (%d,%d): %+v at %d dpi", x, y, centred, practiceDPI)
	}
	// The edit-mode preview is the same click-through window, in the same place.
	if err := setOverlayMode(overlayPreview, s.Settings); err != nil {
		t.Fatal(err)
	}
	style, _, _ = getWindowLong.Call(overlay, ^uintptr(19))
	if visible, _, _ := isVisible.Call(overlay); visible == 0 || style&0x20 == 0 || practiceRect != centred {
		t.Fatalf("preview: style %#x, rect %+v", style, practiceRect)
	}
	// Move mode takes the mouse and honours the saved offset.
	s.Moving, s.Settings.OffsetX, s.Settings.OffsetY = true, 30, -20
	if err := setOverlayMode(overlayMove, s.Settings); err != nil {
		t.Fatal(err)
	}
	if err := renderPractice(s); err != nil {
		t.Fatal(err)
	}
	style, _, _ = getWindowLong.Call(overlay, ^uintptr(19))
	if moved := windowRect(); style&0x20 != 0 || moved.Left != centred.Left+30 || moved.Top != centred.Top-20 {
		t.Fatalf("move mode: style %#x, rect %+v", style, moved)
	}
	hit, _, _ := user32.NewProc("SendMessageW").Call(overlay, 0x0084, 0, 0)
	if hit != 1 {
		t.Fatalf("move mode hit test = %d, want client", hit)
	}
	if err := setOverlayMode(overlayHidden, s.Settings); err != nil {
		t.Fatal(err)
	}
	if visible, _, _ := isVisible.Call(overlay); visible != 0 {
		t.Fatal("practice overlay still visible in edit mode")
	}
}
func TestEmbeddedVoiceClips(t *testing.T) {
	for name, clip := range map[string][]byte{"left": voiceLeft, "right": voiceRight, "natural left": naturalLeft, "natural right": naturalRight, "left tone": toneLeft, "right tone": toneRight} {
		if len(clip) < 44 || string(clip[:4]) != "RIFF" || string(clip[8:12]) != "WAVE" {
			t.Fatalf("invalid %s WAV", name)
		}
		if binary.LittleEndian.Uint16(clip[20:22]) != 1 || binary.LittleEndian.Uint16(clip[22:24]) != 1 || binary.LittleEndian.Uint16(clip[34:36]) != 16 {
			t.Fatalf("%s must be mono PCM16", name)
		}
		rate := binary.LittleEndian.Uint32(clip[28:32])
		dataSize := binary.LittleEndian.Uint32(clip[40:44])
		seconds := float64(dataSize) / float64(rate)
		if seconds <= 0 || seconds > .4 {
			t.Fatalf("unexpected %s clip duration %f", name, seconds)
		}
	}
}
func TestNativeInputLifecycle(t *testing.T) {
	if os.Getenv("RECOIL_NATIVE_TEST") != "1" {
		t.Skip("set RECOIL_NATIVE_TEST=1 for a Windows registration smoke test")
	}
	events := make(chan inputEvent, 256)
	input, err := startNativeInput(events)
	if err != nil {
		t.Fatal(err)
	}
	input.Close()
	select {
	case <-input.done:
	case <-time.After(time.Second):
		t.Fatal("input thread did not stop")
	}
	// The listener must be able to start again after a clean stop.
	second, err := startNativeInput(events)
	if err != nil {
		t.Fatal(err)
	}
	second.Close()
}
