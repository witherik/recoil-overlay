package main

// A controller button, trigger or stick direction is a key too: its code is
// padKey plus its own number. This lets the settings, the key binding and the
// spray logic treat it exactly like a keyboard key or a mouse button.
//
// This file holds only the platform-independent part: the codes, their names,
// and the turning of a raw controller reading into presses and releases. The
// Windows part, which reads the controller, is in gamepad_windows.go.

const padKey = 0x20000

// The codes of the controller inputs. Buttons follow the Xbox layout; a
// PlayStation controller shows up the same way through Steam Input or DS4Windows.
const (
	padA = padKey + 1 + iota
	padB
	padX
	padY
	padLB
	padRB
	padBack
	padStart
	padL3
	padR3
	padDpadUp
	padDpadDown
	padDpadLeft
	padDpadRight
	padLT
	padRT
	padStickLeft  // left stick pushed to the left
	padStickRight // left stick pushed to the right
)

var padNames = map[int]string{
	padA: "Pad A", padB: "Pad B", padX: "Pad X", padY: "Pad Y",
	padLB: "Pad LB", padRB: "Pad RB", padBack: "Pad Back", padStart: "Pad Start",
	padL3: "Pad L3", padR3: "Pad R3",
	padDpadUp: "D-pad up", padDpadDown: "D-pad down", padDpadLeft: "D-pad left", padDpadRight: "D-pad right",
	padLT: "Pad LT", padRT: "Pad RT",
	padStickLeft: "Stick left", padStickRight: "Stick right",
}

// padName returns the label of a controller input, or "" if code is not one.
func padName(code int) string { return padNames[code] }

// The XInput button bits and the code each one stands for.
var padButtons = []struct {
	mask uint16
	code int
}{
	{0x1000, padA}, {0x2000, padB}, {0x4000, padX}, {0x8000, padY},
	{0x0100, padLB}, {0x0200, padRB}, {0x0020, padBack}, {0x0010, padStart},
	{0x0040, padL3}, {0x0080, padR3},
	{0x0001, padDpadUp}, {0x0002, padDpadDown}, {0x0004, padDpadLeft}, {0x0008, padDpadRight},
}

// The analog inputs count as pressed past an "on" level and as released again
// only below a lower "off" level, so a value hovering at the edge cannot flicker.
const (
	triggerOn, triggerOff = 100, 60      // triggers: 0 to 255
	stickOn, stickOff     = 16000, 11000 // stick: 0 to 32767, about half and a third
)

// A padSample is one reading of the controller.
type padSample struct {
	Buttons uint16
	LT, RT  uint8
	LX      int16 // left stick, horizontal
}

// A padChange is one input going down or up.
type padChange struct {
	Code int
	Down bool
}

// A padTracker remembers which inputs are down, to report only the changes.
type padTracker struct {
	down map[int]bool
}

// level applies the hysteresis: was says whether the input was already down.
func level(was bool, value, on, off int) bool {
	if was {
		return value > off
	}
	return value >= on
}

// update returns the inputs that changed since the previous reading.
func (t *padTracker) update(s padSample) []padChange {
	if t.down == nil {
		t.down = map[int]bool{}
	}
	now := map[int]bool{}
	for _, b := range padButtons {
		now[b.code] = s.Buttons&b.mask != 0
	}
	now[padLT] = level(t.down[padLT], int(s.LT), triggerOn, triggerOff)
	now[padRT] = level(t.down[padRT], int(s.RT), triggerOn, triggerOff)
	lx := int(s.LX)
	now[padStickLeft] = level(t.down[padStickLeft], -lx, stickOn, stickOff)
	now[padStickRight] = level(t.down[padStickRight], lx, stickOn, stickOff)

	var changes []padChange
	for code := padA; code <= padStickRight; code++ {
		if now[code] != t.down[code] {
			changes = append(changes, padChange{Code: code, Down: now[code]})
		}
	}
	t.down = now
	return changes
}

// release lets go of everything held, as when the controller is unplugged.
func (t *padTracker) release() []padChange {
	var changes []padChange
	for code := padA; code <= padStickRight; code++ {
		if t.down[code] {
			changes = append(changes, padChange{Code: code, Down: false})
		}
	}
	t.down = nil
	return changes
}
