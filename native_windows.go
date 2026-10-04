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
var registerHotKey = user32.NewProc("RegisterHotKey")
var unregisterHotKey = user32.NewProc("UnregisterHotKey")
var foregroundWindow = user32.NewProc("GetForegroundWindow")
var windowText = user32.NewProc("GetWindowTextW")
var windowPID = user32.NewProc("GetWindowThreadProcessId")
var enumWindows = user32.NewProc("EnumWindows")
var getWindowLong = user32.NewProc("GetWindowLongPtrW")
var setWindowLong = user32.NewProc("SetWindowLongPtrW")
var setWindowPos = user32.NewProc("SetWindowPos")
var playSound = windows.NewLazySystemDLL("winmm.dll").NewProc("PlaySoundW")

// Both slices remain rooted for the entire process lifetime (async PlaySound).
//
//go:embed assets/voice/left.wav
var voiceLeft []byte

//go:embed assets/voice/right.wav
var voiceRight []byte

func playVoice(direction string) {
	clip := voiceRight
	if direction == "left" {
		clip = voiceLeft
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
		overlay, err := createPracticeWindow()
		if err != nil {
			ready <- err
			return
		}
		practiceWindow = overlay
		defer destroyWindow.Call(overlay)
		device := rawDevice{Page: 1, Usage: 2, Flags: 0x100, Target: hwnd} // background mouse only, no suppression
		ok, _, err := registerRaw.Call(uintptr(unsafe.Pointer(&device)), 1, unsafe.Sizeof(device))
		if ok == 0 {
			ready <- fmt.Errorf("register mouse input: %w", err)
			return
		}
		defer func() {
			device.Flags = 1
			device.Target = 0
			registerRaw.Call(uintptr(unsafe.Pointer(&device)), 1, unsafe.Sizeof(device))
		}()
		for i, key := range []uintptr{0x77, 0x78} { // F8 / F9; MOD_NOREPEAT
			ok, _, err = registerHotKey.Call(hwnd, uintptr(i+1), 0x4000, key)
			if ok == 0 {
				ready <- fmt.Errorf("F%d is already in use; close the conflicting app and restart: %w", i+8, err)
				return
			}
			defer unregisterHotKey.Call(hwnd, uintptr(i+1))
		}
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
			if msg.Message == 0x0312 {
				kind := "toggle"
				if msg.WParam == 2 {
					kind = "pause"
				}
				events <- inputEvent{Kind: kind, At: time.Now()}
			}
			if msg.Message == 0x00FF {
				at := time.Now()
				// Fixed aligned buffer covers RAWINPUTHEADER + RAWMOUSE (48 bytes on x64).
				var buffer [16]uint64
				size := uint32(unsafe.Sizeof(buffer))
				count, _, _ := rawData.Call(msg.LParam, 0x10000003, uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)), 24)
				if count >= 48 && count <= unsafe.Sizeof(buffer) && size <= uint32(unsafe.Sizeof(buffer)) {
					bytes := unsafe.Slice((*byte)(unsafe.Pointer(&buffer[0])), int(size))
					if binary.LittleEndian.Uint32(bytes[:4]) == 0 {
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
	return title == "Apex Legends" || title == "Apex Legends™"
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
// practice mode hides it and shows a native layered twin over the same rect.
var mainWindow, practiceWindow uintptr
var practiceRect struct{ Left, Top, Right, Bottom int32 }
var practiceDPI uintptr

// Must be called on a thread that keeps pumping messages (or owns the window).
func createPracticeWindow() (uintptr, error) {
	cls, _ := windows.UTF16PtrFromString("STATIC")
	// layered | transparent | noactivate | toolwindow | topmost; WS_POPUP
	hwnd, _, err := createWindow.Call(0x00080000|0x20|0x08000000|0x80|0x8, uintptr(unsafe.Pointer(cls)), 0, 0x80000000, 0, 0, 0, 0, 0, 0, 0, 0)
	if hwnd == 0 {
		return 0, fmt.Errorf("create practice overlay: %w", err)
	}
	return hwnd, nil
}

// Screen position of the crosshair: the middle of Apex's client area, or of the
// monitor holding our window when the game isn't running.
func crosshairPoint() (int32, int32) {
	for _, name := range []string{"Apex Legends", "Apex Legends™"} {
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

// Moves the main window so the midpoint between the arrows lands on the
// crosshair. Twice, because crossing monitors can change the DPI and size.
func centerOnCrosshair() {
	x, y := crosshairPoint()
	for i := 0; i < 2; i++ {
		var rect struct{ Left, Top, Right, Bottom int32 }
		user32.NewProc("GetWindowRect").Call(mainWindow, uintptr(unsafe.Pointer(&rect)))
		dpi, _, _ := user32.NewProc("GetDpiForWindow").Call(mainWindow)
		if dpi == 0 {
			dpi = 96
		}
		left := x - (rect.Right-rect.Left)/2
		top := y - int32(math.Round(arrowAnchorY*float64(dpi)/96))
		if left == rect.Left && top == rect.Top {
			return
		}
		setWindowPos.Call(mainWindow, 0, uintptr(left), uintptr(top), 0, 0, 0x0001|0x0004|0x0010)
	}
}

func setPracticeWindow(practice, center bool) error {
	if mainWindow == 0 {
		mainWindow = ownWindow()
		if mainWindow == 0 {
			return fmt.Errorf("overlay window not found")
		}
	}
	showWindow := user32.NewProc("ShowWindow")
	// Async for the Wails window: its UI thread may be waiting on our lock.
	showAsync := user32.NewProc("ShowWindowAsync")
	practiceFrameValid = false
	if !practice {
		showWindow.Call(practiceWindow, 0)
		showAsync.Call(mainWindow, 5)
		return nil
	}
	if practiceWindow == 0 {
		return fmt.Errorf("practice overlay unavailable")
	}
	if center {
		centerOnCrosshair()
	}
	user32.NewProc("GetWindowRect").Call(mainWindow, uintptr(unsafe.Pointer(&practiceRect)))
	practiceDPI, _, _ = user32.NewProc("GetDpiForWindow").Call(mainWindow)
	showWindow.Call(practiceWindow, 4) // SW_SHOWNOACTIVATE; invisible until the first frame
	showAsync.Call(mainWindow, 0)
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
