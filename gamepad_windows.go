//go:build windows && amd64

package main

import (
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// xinputState is XINPUT_STATE: a packet number and the XINPUT_GAMEPAD reading.
type xinputState struct {
	Packet         uint32
	Buttons        uint16
	LT, RT         uint8
	LX, LY, RX, RY int16
}

// The XInput libraries, newest first. Windows 10 and 11 have the first one.
var xinputLibraries = []string{"xinput1_4.dll", "xinput1_3.dll", "xinput9_1_0.dll"}

func loadXInput() *windows.LazyProc {
	for _, name := range xinputLibraries {
		proc := windows.NewLazySystemDLL(name).NewProc("XInputGetState")
		if proc.Find() == nil {
			return proc
		}
	}
	return nil
}

const (
	padPollEvery = 2 * time.Millisecond // how often the controller is read
	padScanEvery = time.Second          // how often to look for a controller while none is in use
	padSlots     = 4                    // XInput numbers controllers 0 to 3
)

// gamepad is the XInput listener: a goroutine that reads the first connected
// controller and forwards the bound inputs as key events, like the Raw Input
// listener does for the keyboard and the mouse. Input is only read.
type gamepad struct {
	stop chan struct{}
	done chan struct{}
	once sync.Once
}

// startGamepad starts reading the controller in the background. It returns nil
// when XInput is not available; the keyboard and the mouse still work.
func startGamepad(events chan<- inputEvent) *gamepad {
	proc := loadXInput()
	if proc == nil {
		return nil
	}
	g := &gamepad{stop: make(chan struct{}), done: make(chan struct{})}
	go g.run(proc, events)
	return g
}

func (g *gamepad) run(proc *windows.LazyProc, events chan<- inputEvent) {
	defer close(g.done)

	read := func(slot int) (padSample, bool) {
		var st xinputState
		result, _, _ := proc.Call(uintptr(slot), uintptr(unsafe.Pointer(&st)))
		return padSample{Buttons: st.Buttons, LT: st.LT, RT: st.RT, LX: st.LX}, result == 0
	}
	// forward sends the changes of the watched inputs, or of any input pressed
	// while a binding is being captured. It reports false once stopped.
	forward := func(changes []padChange, at time.Time) bool {
		for _, c := range changes {
			if !watched(uint32(c.Code)) && !(c.Down && captureKey.Load()) {
				continue
			}
			e := inputEvent{Kind: "key", At: at, Code: c.Code, Down: c.Down, Apex: c.Down && apexForeground()}
			select {
			case events <- e:
			case <-g.stop:
				return false
			}
		}
		return true
	}

	tick := time.NewTicker(padPollEvery)
	defer tick.Stop()
	var tracker padTracker
	var lastScan time.Time
	slot := -1
	for {
		select {
		case <-g.stop:
			return
		case <-tick.C:
		}
		if slot < 0 {
			// Asking about a controller that is not there is slow, so look for
			// one only once in a while.
			if time.Since(lastScan) < padScanEvery {
				continue
			}
			lastScan = time.Now()
			for i := 0; i < padSlots && slot < 0; i++ {
				if _, ok := read(i); ok {
					slot = i
				}
			}
			continue
		}
		sample, ok := read(slot)
		at := time.Now()
		if !ok { // unplugged: let go of everything it held
			slot = -1
			if !forward(tracker.release(), at) {
				return
			}
			continue
		}
		if !forward(tracker.update(sample), at) {
			return
		}
	}
}

// Close stops the listener and waits a moment for it to end.
func (g *gamepad) Close() {
	if g == nil {
		return
	}
	g.once.Do(func() {
		close(g.stop)
		select {
		case <-g.done:
		case <-time.After(time.Second):
		}
	})
}
