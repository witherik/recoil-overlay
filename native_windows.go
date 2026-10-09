//go:build windows && amd64

package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"unsafe"

	wr "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

var (
	user32 = windows.NewLazySystemDLL("user32.dll")

	createWindow        = user32.NewProc("CreateWindowExW")
	destroyWindow       = user32.NewProc("DestroyWindow")
	defWindowProc       = user32.NewProc("DefWindowProcW")
	registerRaw         = user32.NewProc("RegisterRawInputDevices")
	rawData             = user32.NewProc("GetRawInputData")
	getMessage          = user32.NewProc("GetMessageW")
	dispatchMessage     = user32.NewProc("DispatchMessageW")
	postThreadMessage   = user32.NewProc("PostThreadMessageW")
	getKeyNameText      = user32.NewProc("GetKeyNameTextW")
	foregroundWindow    = user32.NewProc("GetForegroundWindow")
	findWindow          = user32.NewProc("FindWindowW")
	enumWindows         = user32.NewProc("EnumWindows")
	windowText          = user32.NewProc("GetWindowTextW")
	windowPID           = user32.NewProc("GetWindowThreadProcessId")
	getWindowLong       = user32.NewProc("GetWindowLongPtrW")
	setWindowLong       = user32.NewProc("SetWindowLongPtrW")
	setWindowPos        = user32.NewProc("SetWindowPos")
	getWindowRect       = user32.NewProc("GetWindowRect")
	getClientRect       = user32.NewProc("GetClientRect")
	clientToScreen      = user32.NewProc("ClientToScreen")
	showWindow          = user32.NewProc("ShowWindow")
	showWindowAsync     = user32.NewProc("ShowWindowAsync")
	isIconic            = user32.NewProc("IsIconic")
	updateLayeredWindow = user32.NewProc("UpdateLayeredWindow")
	getCursorPos        = user32.NewProc("GetCursorPos")
	setCapture          = user32.NewProc("SetCapture")
	releaseCapture      = user32.NewProc("ReleaseCapture")
	monitorFromWindow   = user32.NewProc("MonitorFromWindow")
	getMonitorInfo      = user32.NewProc("GetMonitorInfoW")
	getDpiForWindow     = user32.NewProc("GetDpiForWindow")
	getSystemMetrics    = user32.NewProc("GetSystemMetrics")
)

// Win32 values, under their names in the Windows headers.
const (
	wmQuit           = 0x0012
	wmMouseActivate  = 0x0021
	wmNCHitTest      = 0x0084
	wmInput          = 0x00FF
	wmMouseMove      = 0x0200
	wmLButtonDown    = 0x0201
	wmLButtonUp      = 0x0202
	wmCaptureChanged = 0x0215

	wsPopup         = 0x80000000
	wsExTopmost     = 0x00000008
	wsExTransparent = 0x00000020
	wsExToolWindow  = 0x00000080
	wsExLayered     = 0x00080000
	wsExNoActivate  = 0x08000000

	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010
)

// The mouse passes through a click-through window, which never takes focus.
const clickThrough = wsExTransparent | wsExNoActivate

// Indices for GetWindowLongPtr and SetWindowLongPtr: -4 and -20.
const gwlpWndProc, gwlExStyle = ^uintptr(3), ^uintptr(19)

type point struct{ X, Y int32 }

type winMessage struct {
	Window         uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	X, Y           int32
	Private        uint32
}

// The game's window title, with and without its trademark sign.
var apexTitles = []string{"Apex Legends", "Apex Legends™"}

func titleOf(hwnd uintptr) string {
	var text [512]uint16
	windowText.Call(hwnd, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)))
	return windows.UTF16ToString(text[:])
}

// apexForeground reports whether the game is the window being played.
func apexForeground() bool {
	hwnd, _, _ := foregroundWindow.Call()
	title := titleOf(hwnd)
	return title == apexTitles[0] || title == apexTitles[1]
}

// ownWindow finds the settings window, which Wails does not hand out.
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
var practiceRect windows.Rect
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
		var rect windows.Rect
		switch msg {
		case wmNCHitTest:
			return 1 // HTCLIENT, so button messages arrive here
		case wmMouseActivate:
			return 3 // MA_NOACTIVATE
		case wmLButtonDown:
			getCursorPos.Call(uintptr(unsafe.Pointer(&cursor)))
			getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
			overlayGrab = point{cursor.X - rect.Left, cursor.Y - rect.Top}
			overlayDragging = true
			setCapture.Call(hwnd)
			return 0
		case wmMouseMove:
			if overlayDragging {
				getCursorPos.Call(uintptr(unsafe.Pointer(&cursor)))
				setWindowPos.Call(hwnd, 0, uintptr(cursor.X-overlayGrab.X), uintptr(cursor.Y-overlayGrab.Y), 0, 0, swpNoSize|swpNoZOrder|swpNoActivate)
				return 0
			}
		case wmLButtonUp, wmCaptureChanged:
			if overlayDragging {
				overlayDragging = false
				if msg == wmLButtonUp {
					releaseCapture.Call()
				}
				getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
				if overlayMoved != nil {
					overlayMoved(int(rect.Left-practiceBase.X), int(rect.Top-practiceBase.Y))
				}
				return 0
			}
		}
	}
	result, _, _ := defWindowProc.Call(hwnd, msg, wparam, lparam)
	return result
})

// Must be called on a thread that keeps pumping messages (or owns the window).
func createPracticeWindow() (uintptr, error) {
	cls, _ := windows.UTF16PtrFromString("STATIC")
	hwnd, _, err := createWindow.Call(wsExLayered|clickThrough|wsExToolWindow|wsExTopmost, uintptr(unsafe.Pointer(cls)), 0, wsPopup, 0, 0, 0, 0, 0, 0, 0, 0)
	if hwnd == 0 {
		return 0, fmt.Errorf("create practice overlay: %w", err)
	}
	setWindowLong.Call(hwnd, gwlpWndProc, overlayProc)
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
	for _, name := range apexTitles {
		title, _ := windows.UTF16PtrFromString(name)
		hwnd, _, _ := findWindow.Call(0, uintptr(unsafe.Pointer(title)))
		if minimized, _, _ := isIconic.Call(hwnd); hwnd == 0 || minimized != 0 {
			continue
		}
		var client windows.Rect
		getClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
		middle := point{client.Right / 2, client.Bottom / 2}
		if ok, _, _ := clientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&middle))); ok != 0 && client.Right > 0 {
			return middle.X, middle.Y
		}
	}
	var info struct {
		Size          uint32
		Monitor, Work windows.Rect
		Flags         uint32
	}
	info.Size = uint32(unsafe.Sizeof(info))
	monitor, _, _ := monitorFromWindow.Call(mainWindow, 2) // MONITOR_DEFAULTTONEAREST
	getMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info)))
	return (info.Monitor.Left + info.Monitor.Right) / 2, (info.Monitor.Top + info.Monitor.Bottom) / 2
}

// desktopRect is the rectangle that spans every monitor.
func desktopRect() windows.Rect {
	metric := func(index uintptr) int32 {
		value, _, _ := getSystemMetrics.Call(index)
		return int32(value)
	}
	// SM_XVIRTUALSCREEN, SM_YVIRTUALSCREEN, SM_CXVIRTUALSCREEN, SM_CYVIRTUALSCREEN
	left, top, width, height := metric(76), metric(77), metric(78), metric(79)
	return windows.Rect{Left: left, Top: top, Right: left + width, Bottom: top + height}
}

// Puts the midpoint between the arrows on the crosshair, shifted by the user's
// saved offset. An offset that would leave the desktop is ignored.
func placeOverlay(s Settings) {
	practiceDPI, _, _ = getDpiForWindow.Call(mainWindow)
	if practiceDPI == 0 {
		practiceDPI = 96
	}
	scale := float64(practiceDPI) / 96
	width, height := overlaySize(s, scale)
	x, y := crosshairPoint()
	practiceBase = point{x - width/2, y - int32(math.Round(anchorY(s)*scale))}
	left, top := practiceBase.X+int32(s.OffsetX), practiceBase.Y+int32(s.OffsetY)
	if desktop := desktopRect(); left+width <= desktop.Left || top+height <= desktop.Top || left >= desktop.Right || top >= desktop.Bottom {
		left, top = practiceBase.X, practiceBase.Y
	}
	practiceRect = windows.Rect{Left: left, Top: top, Right: left + width, Bottom: top + height}
	practicePlaced, practiceFrameValid = false, false
}

// setOverlayMode shows the overlay in the given mode, placed by the settings,
// and the settings window to match: minimised for practice, visible otherwise.
// The overlay stays invisible until renderPractice gives it its first frame.
func setOverlayMode(mode int, s Settings) error {
	if mainWindow == 0 {
		mainWindow = ownWindow()
		if mainWindow == 0 {
			return fmt.Errorf("overlay window not found")
		}
	}
	// The Wails window is only ever shown asynchronously: its UI thread may be
	// waiting on our lock.
	overlayMovable.Store(mode == overlayMove)
	if mode == overlayHidden {
		showWindow.Call(practiceWindow, windows.SW_HIDE)
		showWindowAsync.Call(mainWindow, windows.SW_SHOW)
		return nil
	}
	if practiceWindow == 0 {
		return fmt.Errorf("practice overlay unavailable")
	}
	placeOverlay(s)
	extended, _, _ := getWindowLong.Call(practiceWindow, gwlExStyle)
	if mode == overlayMove {
		extended &^= clickThrough
	} else {
		extended |= clickThrough
	}
	setWindowLong.Call(practiceWindow, gwlExStyle, extended)
	showWindow.Call(practiceWindow, windows.SW_SHOWNOACTIVATE)
	// Above the settings panel too, so the overlay is never hidden behind it.
	const topmost = ^uintptr(0) // HWND_TOPMOST
	setWindowPos.Call(practiceWindow, topmost, 0, 0, 0, 0, swpNoSize|swpNoMove|swpNoActivate)
	switch mode {
	case overlayPractice:
		// Minimised rather than hidden, so the app keeps its taskbar button.
		showWindowAsync.Call(mainWindow, windows.SW_MINIMIZE)
	case overlayPreview:
		showWindowAsync.Call(mainWindow, windows.SW_SHOW)
	}
	return nil
}

// mainMinimised reports whether the settings window is in the taskbar.
func mainMinimised() bool {
	if mainWindow == 0 {
		return false
	}
	minimised, _, _ := isIconic.Call(mainWindow)
	return minimised != 0
}

// restoreMainWindow brings the settings window back from the taskbar. Async:
// its UI thread may be waiting on our lock.
func restoreMainWindow() {
	if mainMinimised() {
		showWindowAsync.Call(mainWindow, windows.SW_RESTORE)
	}
}

// restorePosition puts the settings window back where it was last time, or in
// the middle of the screen if that place is no longer on the desktop.
func restorePosition(ctx context.Context, x, y int) {
	if desktop := desktopRect(); x < int(desktop.Left) || y < int(desktop.Top) || x > int(desktop.Right)-100 || y > int(desktop.Bottom)-100 {
		wr.WindowCenter(ctx)
		return
	}
	// Wails v2 SetPosition adds the current monitor's work-area origin; our saved
	// WindowGetPosition values are already absolute screen coordinates.
	if hwnd := ownWindow(); hwnd != 0 {
		setWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), 0, 0, swpNoSize|swpNoZOrder|swpNoActivate)
	}
}
