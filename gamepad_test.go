package main

import "testing"

func TestPadTrackerTriggerAndStick(t *testing.T) {
	var tr padTracker
	// Nothing pressed: no change.
	if c := tr.update(padSample{}); len(c) != 0 {
		t.Fatalf("idle: %v", c)
	}
	// Right trigger pulled past the "on" level.
	c := tr.update(padSample{RT: 200})
	if len(c) != 1 || c[0] != (padChange{padRT, true}) {
		t.Fatalf("RT down: %v", c)
	}
	// Eased back but still above the "off" level: stays down, no flicker.
	if c := tr.update(padSample{RT: 80}); len(c) != 0 {
		t.Fatalf("RT hysteresis: %v", c)
	}
	// Below the "off" level: released.
	c = tr.update(padSample{RT: 30})
	if len(c) != 1 || c[0] != (padChange{padRT, false}) {
		t.Fatalf("RT up: %v", c)
	}
	// Stick pushed left, then straight through to the right.
	c = tr.update(padSample{LX: -30000})
	if len(c) != 1 || c[0] != (padChange{padStickLeft, true}) {
		t.Fatalf("stick left: %v", c)
	}
	c = tr.update(padSample{LX: 30000})
	if len(c) != 2 || c[0] != (padChange{padStickLeft, false}) || c[1] != (padChange{padStickRight, true}) {
		t.Fatalf("stick left to right: %v", c)
	}
	// A small push (drift) never counts.
	tr = padTracker{}
	if c := tr.update(padSample{LX: 6000}); len(c) != 0 {
		t.Fatalf("drift: %v", c)
	}
	// The most negative reading must not overflow.
	if c := tr.update(padSample{LX: -32768}); len(c) != 1 || c[0].Code != padStickLeft {
		t.Fatalf("min stick: %v", c)
	}
}

func TestPadTrackerButtonsAndRelease(t *testing.T) {
	var tr padTracker
	c := tr.update(padSample{Buttons: 0x1000 | 0x0020}) // A and Back
	if len(c) != 2 || c[0] != (padChange{padA, true}) || c[1] != (padChange{padBack, true}) {
		t.Fatalf("buttons: %v", c)
	}
	if c := tr.update(padSample{Buttons: 0x1000 | 0x0020}); len(c) != 0 {
		t.Fatalf("held buttons repeat: %v", c)
	}
	c = tr.release() // unplugged
	if len(c) != 2 || c[0].Down || c[1].Down {
		t.Fatalf("release: %v", c)
	}
	if c := tr.release(); len(c) != 0 {
		t.Fatalf("second release: %v", c)
	}
}

func TestPadNames(t *testing.T) {
	for code := padA; code <= padStickRight; code++ {
		if padName(code) == "" {
			t.Errorf("code %#x has no name", code)
		}
	}
	if padName(leftMouse) != "" || padName(0x1E) != "" {
		t.Error("keyboard and mouse codes must not be taken for controller inputs")
	}
}
