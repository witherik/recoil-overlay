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
	a := &App{settings: defaultSettings(), armed: true, inputReady: true, speak: func(_, s string) { spoken = append(spoken, s) }, silence: func() {}}
	a.settings.VoiceLeadMS = 150
	a.settings.VoiceStart = true
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
	if s.Score == nil || s.Score.TotalMS != 50 || s.Score.Missed != 0 || s.Score.Switches[1].DeviationMS != 20 || s.Score.Switches[2].DeviationMS != 30 {
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

func TestWeaponPatternsDriveCuesAndScore(t *testing.T) {
	a, spoken := testApp()
	a.settings.WeaponID, a.settings.ModeID = "havoc", "normal"
	at := time.Now()
	a.handleInputLocked(inputEvent{Kind: "down", At: at, Apex: true})
	if s := a.snapshotLocked(); s.Weapon != "HAVOC" || s.Mode != "NORMAL" || s.TotalMS != 2850 || s.Direction != "" || len(*spoken) != 0 {
		t.Fatalf("the charge-up is neutral and silent: %+v %v", s, *spoken)
	}
	a.advanceLocked(at.Add(199*time.Millisecond), true)
	if len(*spoken) != 0 {
		t.Fatal(*spoken)
	}
	a.advanceLocked(at.Add(200*time.Millisecond), true) // 350 ms switch minus the 150 ms lead
	if !reflect.DeepEqual(*spoken, []string{"right"}) {
		t.Fatal(*spoken)
	}
	a.handleInputLocked(inputEvent{Kind: "key", Code: a.settings.RightKey, Down: true, At: at.Add(380 * time.Millisecond)})
	a.handleInputLocked(inputEvent{Kind: "up", At: at.Add(600 * time.Millisecond)})
	s := a.snapshotLocked()
	if s.Score == nil || len(s.Score.Switches) != 1 || s.Score.Switches[0].AtMS != 350 || s.Score.Switches[0].DeviationMS != 30 {
		t.Fatalf("score: %+v", s.Score)
	}
	// Switching weapon drops the review, which was scored against the old pattern.
	a.editing = true
	next := a.settings
	next.WeaponID, next.ModeID = "r99", ""
	if _, err := a.UpdateSettings(next); err != nil {
		t.Fatal(err)
	}
	if s := a.snapshotLocked(); s.Score != nil || s.Weapon != "R-99" || s.Settings.ModeID != "default" || s.Mode != "EXPECTED STRAFE" || s.TotalMS != 1450 {
		t.Fatalf("after switching weapon: %+v", s)
	}
}

func TestOpeningStrafeIsSilentByDefault(t *testing.T) {
	if defaultSettings().VoiceStart {
		t.Fatal("the opening cue should be opt-in")
	}
	for _, test := range []struct {
		weapon, mode string
		atMS         int
		want         []string
	}{
		{"r301", "default", 700, []string{"left"}},       // not "right" at the click
		{"havoc", "normal", 600, []string{"left"}},       // nor "right" after the wait
		{"havoc", "turbocharged", 300, []string{"left"}}, // 370 ms switch minus the lead
		{"havoc", "turbocharged", 219, nil},
	} {
		a, spoken := testApp()
		a.settings.VoiceStart = false
		a.settings.WeaponID, a.settings.ModeID = test.weapon, test.mode
		at := time.Now()
		a.handleInputLocked(inputEvent{Kind: "down", At: at, Apex: true})
		for ms := 10; ms <= test.atMS; ms += 10 {
			a.advanceLocked(at.Add(time.Duration(ms)*time.Millisecond), true)
		}
		a.advanceLocked(at.Add(time.Duration(test.atMS)*time.Millisecond), true)
		if !reflect.DeepEqual(append([]string(nil), *spoken...), test.want) {
			t.Errorf("%s/%s at %d ms: spoke %v, want %v", test.weapon, test.mode, test.atMS, *spoken, test.want)
		}
	}
}

func TestPracticeKeys(t *testing.T) {
	a, _ := testApp()
	a.editing = true
	key := func(code int, down bool) { a.handleInputLocked(inputEvent{Kind: "key", Code: code, Down: down}) }
	pause := a.settings.PauseKey
	key(pause, true)
	key(pause, true) // auto-repeat of a held key
	if a.armed {
		t.Fatal("the pause key should disable once per press")
	}
	key(pause, false)
	key(pause, true)
	if !a.armed {
		t.Fatal("a fresh press should enable again")
	}
	key(pause, false)
	// A key held by an unrelated action is refused, and the binding keeps waiting.
	a.binding = "start"
	key(pause, true)
	key(a.settings.LeftKey, true)
	if a.binding != "start" || a.settings.StartKey != 0x42 || !a.armed {
		t.Fatalf("taken keys must be refused: %+v", a.settings)
	}
	key(0x41, true) // F7: start and end may then differ
	key(0x41, true) // its auto-repeat must not start practice
	if a.binding != "" || a.settings.StartKey != 0x41 || a.settings.EndKey != 0x42 || !a.editing || a.err != "" {
		t.Fatalf("bind start: %+v %q", a.settings, a.err)
	}
	a.binding = "end"
	key(0x41, true) // sharing the start key makes it a toggle
	if a.settings.EndKey != 0x41 || a.settings.StartKey != 0x41 {
		t.Fatalf("shared key: %+v", a.settings)
	}
}

func TestVoiceStyleMovesDefaultLead(t *testing.T) {
	a, spoken := testApp()
	a.editing = true
	a.speak = func(style, direction string) { *spoken = append(*spoken, style+" "+direction) }
	next := defaultSettings()
	next.VoiceStyle = "natural"
	a.settings = defaultSettings()
	if s, _ := a.UpdateSettings(next); s.Settings.VoiceLeadMS != voiceLeads["natural"] {
		t.Fatalf("lead should follow the style: %d", s.Settings.VoiceLeadMS)
	}
	next = a.settings
	next.VoiceLeadMS, next.VoiceStyle = 120, "tones"
	if s, _ := a.UpdateSettings(next); s.Settings.VoiceLeadMS != 120 {
		t.Fatalf("a chosen lead should be kept: %d", s.Settings.VoiceLeadMS)
	}
	next.VoiceStyle = "bogus"
	if s, _ := a.UpdateSettings(next); s.Settings.VoiceStyle != "fast" {
		t.Fatalf("unknown style: %q", s.Settings.VoiceStyle)
	}
	a.settings.VoiceStyle, a.settings.VoiceStart, a.editing = "tones", true, false
	a.handleInputLocked(inputEvent{Kind: "down", At: time.Now(), Apex: true})
	if !reflect.DeepEqual(*spoken, []string{"tones right"}) {
		t.Fatal(*spoken)
	}
}

func TestMinimisingHidesOnlyThePreview(t *testing.T) {
	a, _ := testApp()
	a.editing, a.shown, a.moving = true, true, true
	a.minimiseLocked(true)
	if a.overlayModeLocked() != overlayHidden || a.moving {
		t.Fatal("a minimised editor should take its preview along")
	}
	a.editing = false
	if a.overlayModeLocked() != overlayPractice {
		t.Fatal("practice must keep the overlay")
	}
	a.editing = true
	a.minimiseLocked(false)
	if a.overlayModeLocked() != overlayPreview {
		t.Fatal("restoring the editor should bring the preview back")
	}
}

func TestResetDefaultsKeepsTheWindowInPlace(t *testing.T) {
	a, _ := testApp()
	a.editing = true
	a.settings.Gap, a.settings.Theme, a.settings.StartKey, a.settings.OffsetX = 200, "blue", 0x41, 50
	a.settings.X, a.settings.Width, a.settings.Positioned = 300, 800, true
	want := defaultSettings()
	want.X, want.Width, want.Positioned = 300, 800, true
	if got := a.ResetDefaults().Settings; got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
	a.editing = false
	a.settings.Gap = 200
	if a.ResetDefaults().Settings.Gap != 200 {
		t.Fatal("practice mode must not reset")
	}
}
