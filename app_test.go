package main

import (
	"reflect"
	"testing"
	"time"
)

func testApp() (*App, *[]string) {
	spoken := []string{}
	a := &App{settings: defaultSettings(), armed: true, inputReady: true, speak: func(s string) { spoken = append(spoken, s) }, silence: func() {}}
	return a, &spoken
}
func TestHoldReleaseCancelsFutureCues(t *testing.T) {
	a, spoken := testApp()
	at := time.Now()
	a.handleInputLocked(inputEvent{Kind: "down", At: at, Apex: true})
	a.advanceLocked(at.Add(649*time.Millisecond), true)
	if !reflect.DeepEqual(*spoken, []string{"right"}) {
		t.Fatal(*spoken)
	}
	a.handleInputLocked(inputEvent{Kind: "up", At: at.Add(650 * time.Millisecond)})
	a.advanceLocked(at.Add(1400*time.Millisecond), true)
	if a.running || a.held || a.elapsed != 0 || len(*spoken) != 1 {
		t.Fatalf("release did not cancel: %+v, %v", a.snapshotLocked(), *spoken)
	}
	a.handleInputLocked(inputEvent{Kind: "down", At: at.Add(2 * time.Second), Apex: true})
	if !a.running || a.runID != 2 {
		t.Fatal("fresh click did not restart")
	}
}
func TestVoiceLeadAndExactVisualBoundaries(t *testing.T) {
	a, spoken := testApp()
	at := time.Now()
	a.startLocked(at, false)
	a.advanceLocked(at.Add(650*time.Millisecond), true)
	if a.snapshotLocked().Direction != "right" || !reflect.DeepEqual(*spoken, []string{"right", "left"}) {
		t.Fatal("voice should lead visual transition")
	}
	a.advanceLocked(at.Add(800*time.Millisecond), true)
	if a.snapshotLocked().Direction != "left" {
		t.Fatal("800ms should be left")
	}
	a.advanceLocked(at.Add(1180*time.Millisecond), true)
	if !reflect.DeepEqual(*spoken, []string{"right", "left", "right"}) {
		t.Fatal(*spoken)
	}
	a.advanceLocked(at.Add(1330*time.Millisecond), true)
	if a.snapshotLocked().Direction != "right" {
		t.Fatal("1330ms should be right")
	}
	a.advanceLocked(at.Add(2210*time.Millisecond), true)
	if a.running || a.elapsed != 2210 {
		t.Fatal("pattern must stop at 2210ms")
	}
	a.advanceLocked(at.Add(4*time.Second), true)
	if a.running || len(*spoken) != 3 {
		t.Fatal("pattern must not loop")
	}
}
func TestStartGatesAndFreshPressAfterPause(t *testing.T) {
	for _, mode := range []string{"editing", "disabled", "unfocused", "unavailable"} {
		t.Run(mode, func(t *testing.T) {
			a, _ := testApp()
			e := inputEvent{Kind: "down", At: time.Now(), Apex: true}
			switch mode {
			case "editing":
				a.editing = true
			case "disabled":
				a.armed = false
			case "unfocused":
				e.Apex = false
			case "unavailable":
				a.inputReady = false
			}
			a.handleInputLocked(e)
			if a.running {
				t.Fatal("should not start")
			}
		})
	}
	a, _ := testApp()
	e := inputEvent{Kind: "down", At: time.Now(), Apex: true}
	a.handleInputLocked(e)
	a.handleInputLocked(e)
	if a.runID != 1 {
		t.Fatal("repeat down restarted spray")
	}
	a.handleInputLocked(inputEvent{Kind: "pause"})
	a.handleInputLocked(inputEvent{Kind: "pause"})
	a.handleInputLocked(e)
	if a.running {
		t.Fatal("reenabling while held must not restart")
	}
	a.handleInputLocked(inputEvent{Kind: "up"})
	a.handleInputLocked(e)
	if !a.running {
		t.Fatal("fresh press should start")
	}
}
func TestFocusLossAndPreview(t *testing.T) {
	a, _ := testApp()
	at := time.Now()
	a.startLocked(at, false)
	a.advanceLocked(at.Add(100*time.Millisecond), false)
	if a.running {
		t.Fatal("focus loss must cancel")
	}
	a.startLocked(at, true)
	a.advanceLocked(at.Add(800*time.Millisecond), false)
	if !a.running || a.snapshotLocked().Direction != "left" {
		t.Fatal("preview should work outside Apex")
	}
}
