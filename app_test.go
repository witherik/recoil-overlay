package main

import (
	"os"
	"recoil-overlay/internal/pattern"
	"reflect"
	"testing"
	"time"
)

// Tests that save settings must never touch the user's real settings file.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "recoil-overlay-test")
	if err != nil {
		panic(err)
	}
	os.Setenv("APPDATA", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func testApp() (*App, *[]string) {
	spoken := []string{}
	a := &App{settings: defaultSettings(), armed: true, inputReady: true, speak: func(s string) { spoken = append(spoken, s) }, silence: func() {}}
	a.settings.VoiceLeadMS = 150
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

func TestStrafeTrackAndReview(t *testing.T) {
	a, _ := testApp()
	at := time.Now()
	key := func(code int, down bool, ms int) {
		a.handleInputLocked(inputEvent{Kind: "key", Code: code, Down: down, At: at.Add(time.Duration(ms) * time.Millisecond)})
	}
	left, right := a.settings.LeftKey, a.settings.RightKey
	key(right, true, -200) // already strafing when the spray starts
	a.handleInputLocked(inputEvent{Kind: "down", At: at, Apex: true})
	key(right, true, 30) // auto-repeat changes nothing
	key(left, true, 780) // both held: neutral
	key(right, false, 820)
	a.advanceLocked(at.Add(1000*time.Millisecond), true)
	if live := a.snapshotLocked(); live.Score != nil || live.PlayerEndMS != 1000 || len(live.Player) != 3 {
		t.Fatalf("live snapshot: %+v", live)
	}
	key(left, false, 1340)
	key(right, true, 1360)
	a.handleInputLocked(inputEvent{Kind: "up", At: at.Add(1900 * time.Millisecond)})

	s := a.snapshotLocked()
	seg := func(from, to int64, direction string) pattern.Segment {
		return pattern.Segment{StartMS: from, EndMS: to, Direction: direction}
	}
	want := []pattern.Segment{seg(0, 780, "right"), seg(780, 820, ""), seg(820, 1340, "left"), seg(1340, 1360, ""), seg(1360, 1900, "right")}
	if !reflect.DeepEqual(s.Player, want) || s.PlayerEndMS != 1900 || s.ElapsedMS != 0 {
		t.Fatalf("review track: %+v end %d", s.Player, s.PlayerEndMS)
	}
	if s.Score == nil || s.Score.AverageMS != 17 || s.Score.Missed != 0 || s.Score.Switches[1].DeviationMS != 20 || s.Score.Switches[2].DeviationMS != 30 {
		t.Fatalf("score: %+v", s.Score)
	}
	// A tap does not wipe the review; a real spray replaces it.
	a.handleInputLocked(inputEvent{Kind: "down", At: at.Add(3 * time.Second), Apex: true})
	a.handleInputLocked(inputEvent{Kind: "up", At: at.Add(3100 * time.Millisecond)})
	if got := a.snapshotLocked(); got.PlayerEndMS != 1900 {
		t.Fatalf("tap replaced the review: %+v", got)
	}
	a.handleInputLocked(inputEvent{Kind: "down", At: at.Add(4 * time.Second), Apex: true})
	a.advanceLocked(at.Add(7*time.Second), true)
	if got := a.snapshotLocked(); got.PlayerEndMS != 2210 || got.Score == nil || got.Score.Missed != 2 || got.Running || got.Player[0].EndMS != 2210 {
		t.Fatalf("completed spray: %+v %+v", got, got.Score)
	}
}

func TestBindKey(t *testing.T) {
	a, _ := testApp()
	a.editing = true
	left, right := a.settings.LeftKey, a.settings.RightKey
	bind := func(side string, code int) {
		a.binding = side
		a.handleInputLocked(inputEvent{Kind: "key", Code: code, Down: true})
	}
	bind("left", 0x01) // Escape cancels
	if a.binding != "" || a.settings.LeftKey != left {
		t.Fatal("escape should cancel")
	}
	bind("left", 0xE04B)
	if a.settings.LeftKey != 0xE04B || a.settings.RightKey != right || a.leftDown {
		t.Fatalf("bind left: %+v", a.settings)
	}
	bind("right", 0xE04B) // taking the other side's key swaps them
	if a.settings.RightKey != 0xE04B || a.settings.LeftKey != right {
		t.Fatalf("swap: %+v", a.settings)
	}
}
