package main

import (
	"context"
	"fmt"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
	"recoil-overlay/internal/pattern"
	"strings"
	"sync"
	"time"
)

const windowTitle = "Recoil Practice"

type Snapshot struct {
	Settings   Settings        `json:"settings"`
	Phases     []pattern.Phase `json:"phases"`
	Weapon     string          `json:"weapon"`
	Mode       string          `json:"mode"` // shown beside the weapon name
	Editing    bool            `json:"editing"`
	Armed      bool            `json:"armed"`
	Focused    bool            `json:"focused"`
	InputReady bool            `json:"inputReady"`
	Running    bool            `json:"running"`
	Preview    bool            `json:"preview"`
	Moving     bool            `json:"moving"`
	Shown      bool            `json:"shown"` // the overlay is on screen while editing
	Held       bool            `json:"held"`
	ElapsedMS  int64           `json:"elapsedMs"`
	TotalMS    int             `json:"totalMs"`
	Phase      int             `json:"phase"`
	Direction  string          `json:"direction"`
	RunID      uint64          `json:"runId"`
	Error      string          `json:"error"`

	// The player's own strafes: live while a spray runs, then the last spray.
	Player      []pattern.Segment `json:"player"`
	PlayerEndMS int64             `json:"playerEndMs"`
	Score       *pattern.Score    `json:"score"`
	LeftKey     string            `json:"leftKey"`
	RightKey    string            `json:"rightKey"`
	StartKey    string            `json:"startKey"`
	EndKey      string            `json:"endKey"`
	PauseKey    string            `json:"pauseKey"`
	Binding     string            `json:"binding"`
	revision    uint64
}

// A spray shorter than this is a tap, and does not replace the last review.
const minReviewMS = 250

type review struct {
	track []pattern.Segment
	endMS int64
	score pattern.Score
}
type App struct {
	mu                                                          sync.Mutex
	ctx                                                         context.Context
	settings                                                    Settings
	input                                                       *nativeInput
	events                                                      chan inputEvent
	stop                                                        chan struct{}
	done                                                        chan struct{}
	editing, armed, focused, inputReady, running, preview, held bool
	moving                                                      bool
	shown                                                       bool // show the overlay while editing too
	minimised                                                   bool // the settings window is in the taskbar
	leftDown, rightDown                                         bool
	binding                                                     string // the action waiting for a key, see keySlotsLocked
	hotDown                                                     map[int]bool
	track                                                       []pattern.Segment
	last                                                        *review
	revision                                                    uint64
	weapon                                                      pattern.Weapon
	mode                                                        pattern.Mode
	started                                                     time.Time
	elapsed                                                     int64
	runID                                                       uint64
	nextCue                                                     int
	firstCue                                                    int // cue of the spray's first strafe
	cues                                                        []pattern.Cue
	err                                                         string
	speak                                                       func(style, direction string)
	silence                                                     func()
}

func NewApp() *App {
	return &App{settings: readSettings(), editing: true, armed: true, events: make(chan inputEvent, 256), stop: make(chan struct{}), done: make(chan struct{}), speak: playVoice, silence: stopVoice}
}
func (a *App) startup(ctx context.Context) { a.ctx = ctx }
func (a *App) ready(ctx context.Context) {
	a.mu.Lock()
	s := a.settings
	a.mu.Unlock()
	wr.WindowSetSize(ctx, s.Width, s.Height)
	if s.Positioned {
		restorePosition(ctx, s.X, s.Y)
	}
	a.syncKeysLocked()
	overlayMoved = func(x, y int) { a.events <- inputEvent{Kind: "moved", X: x, Y: y} }
	err := startOverlayWindow()
	var in *nativeInput
	if err == nil {
		in, err = startNativeInput(a.events)
	}
	a.mu.Lock()
	a.input = in
	a.inputReady = err == nil
	if err != nil {
		a.err = err.Error()
	}
	// Edit mode starts with the real overlay on screen, as its own preview.
	a.shown = true
	if overlayErr := a.applyOverlayLocked(); overlayErr != nil {
		a.shown = false
		if a.err == "" {
			a.err = overlayErr.Error()
		}
	}
	a.mu.Unlock()
	go a.loop()
}
func (a *App) shutdown(ctx context.Context) {
	close(a.stop)
	if a.input != nil {
		a.input.Close()
	}
	select {
	case <-a.done:
	case <-time.After(time.Second):
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.silence()
	if err := writeSettings(a.settings); err != nil {
		wr.LogError(ctx, err.Error())
	}
}
func (a *App) loop() {
	defer close(a.done)
	tick := time.NewTicker(16 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-a.stop:
			return
		case e := <-a.events:
			a.mu.Lock()
			a.handleInputLocked(e)
			a.emitLocked()
			a.mu.Unlock()
		case now := <-tick.C:
			a.mu.Lock()
			a.advanceLocked(now, apexForeground())
			a.minimiseLocked(mainMinimised())
			a.emitLocked()
			a.mu.Unlock()
		}
	}
}
func (a *App) handleInputLocked(e inputEvent) {
	switch e.Kind {
	case "pause":
		a.armed = !a.armed
		a.cancelLocked()
	case "down":
		if !a.held {
			a.held = true
			if !a.editing && a.armed && a.inputReady && e.Apex {
				a.startLocked(e.At, false)
			}
		}
	case "up":
		a.held = false
		if !a.preview {
			if a.running && !e.At.IsZero() {
				a.elapsed = a.clockLocked(e.At)
			}
			a.cancelLocked()
		}
	case "key":
		if !e.Down {
			delete(a.hotDown, e.Code)
		}
		if a.binding != "" {
			if e.Down {
				a.bindLocked(e.Code)
			}
			return
		}
		if e.Code == a.settings.LeftKey {
			a.leftDown = e.Down
		}
		if e.Code == a.settings.RightKey {
			a.rightDown = e.Down
		}
		a.hotkeyLocked(e.Code, e.Down)
		if a.running {
			a.strafeLocked(a.clockLocked(e.At))
		}
	case "moved":
		if a.moving {
			a.settings.OffsetX, a.settings.OffsetY = e.X, e.Y
			a.saveLocked()
		}
	case "error":
		a.err = e.Detail
		a.inputReady = false
		a.cancelLocked()
		// A failed input thread must never strand the user behind a click-through window.
		if !a.editing {
			if err := setOverlayMode(overlayHidden, a.settings); err == nil {
				a.editing, a.shown = true, false
			}
		}
	}
}
func (a *App) advanceLocked(now time.Time, focused bool) {
	a.focused = focused
	if !focused && !a.preview {
		a.cancelLocked()
	}
	if !a.running {
		return
	}
	a.elapsed = now.Sub(a.started).Milliseconds()
	if a.elapsed < 0 {
		a.elapsed = 0
	}
	if a.elapsed >= int64(pattern.TotalMS(a.patternLocked())) {
		a.elapsed = int64(pattern.TotalMS(a.patternLocked()))
		a.finishLocked(a.elapsed)
		a.running = false
		a.silence()
		return
	}
	a.strafeLocked(a.elapsed)
	// If the process was stalled, announce only the latest due cue, never a backlog.
	due := -1
	for a.nextCue < len(a.cues) && a.elapsed >= a.cues[a.nextCue].AtMS {
		due = a.nextCue
		a.nextCue++
	}
	// Neutral phases (a charge-up before the first strafe) have no voice line,
	// and the opening strafe is announced only if asked for.
	if due >= 0 && a.settings.Voice && a.cues[due].Direction != "" && (due != a.firstCue || a.settings.VoiceStart) {
		a.speak(a.settings.VoiceStyle, a.cues[due].Direction)
	}
}

// hotkeyLocked acts on the practice keys. Raw Input repeats a held key, so
// each one fires only on a fresh press.
func (a *App) hotkeyLocked(code int, down bool) {
	s := a.settings
	if !down || a.hotDown[code] || (code != s.StartKey && code != s.EndKey && code != s.PauseKey) {
		return
	}
	a.holdLocked(code)
	switch {
	case code == s.PauseKey:
		a.handleInputLocked(inputEvent{Kind: "pause"})
	case a.editing && code == s.StartKey, !a.editing && code == s.EndKey:
		a.toggleLocked()
	}
}
func (a *App) holdLocked(code int) {
	if a.hotDown == nil {
		a.hotDown = map[int]bool{}
	}
	a.hotDown[code] = true
}
func (a *App) startLocked(at time.Time, preview bool) {
	a.cancelLocked()
	a.runID++
	a.started = at
	a.running = true
	a.preview = preview
	a.elapsed = 0
	a.track = []pattern.Segment{{Direction: a.strafeDirection()}}
	a.revision++
	a.cues = pattern.Cues(a.patternLocked(), a.settings.VoiceLeadMS)
	a.nextCue = 0
	a.firstCue = 0
	for a.firstCue < len(a.cues)-1 && a.cues[a.firstCue].Direction == "" {
		a.firstCue++
	}
	// The first cue cannot lead: the click is not predictable.
	if a.settings.Voice && a.settings.VoiceStart && a.cues[0].Direction != "" {
		a.speak(a.settings.VoiceStyle, a.cues[0].Direction)
	}
	a.nextCue = 1
}

// patternLocked returns the phases of the selected weapon and mode.
func (a *App) patternLocked() []pattern.Phase {
	if a.mode.Phases == nil || a.weapon.ID != a.settings.WeaponID || a.mode.ID != a.settings.ModeID {
		a.weapon, a.mode = pattern.Find(a.settings.WeaponID, a.settings.ModeID)
		a.settings.WeaponID, a.settings.ModeID = a.weapon.ID, a.mode.ID
	}
	return a.mode.Phases
}

// Weapons lists every preset for the settings panel.
func (a *App) Weapons() []pattern.Weapon { return pattern.Weapons() }

// clockLocked converts a capture time to milliseconds into the current spray.
func (a *App) clockLocked(at time.Time) int64 {
	return min(max(at.Sub(a.started).Milliseconds(), 0), int64(pattern.TotalMS(a.patternLocked())))
}

// strafeDirection is where the held keys move the player; both or neither is neutral.
func (a *App) strafeDirection() string {
	switch {
	case a.leftDown && !a.rightDown:
		return "left"
	case a.rightDown && !a.leftDown:
		return "right"
	}
	return ""
}

// strafeLocked extends the player's track to ms, opening a new segment when
// the strafe direction has changed.
func (a *App) strafeLocked(ms int64) {
	if len(a.track) == 0 {
		return
	}
	current := &a.track[len(a.track)-1]
	current.EndMS = max(current.EndMS, ms)
	if direction := a.strafeDirection(); direction != current.Direction {
		a.track = append(a.track, pattern.Segment{StartMS: current.EndMS, EndMS: current.EndMS, Direction: direction})
		a.revision++
	}
}

// finishLocked closes the spray at endMS and keeps it for review.
func (a *App) finishLocked(endMS int64) {
	a.strafeLocked(endMS)
	if endMS >= minReviewMS {
		a.last = &review{track: a.track, endMS: endMS, score: pattern.Evaluate(a.patternLocked(), a.track, endMS)}
	}
	a.track = nil
	a.revision++
}
func (a *App) syncKeysLocked() {
	s := a.settings
	for i, code := range []int{s.LeftKey, s.RightKey, s.StartKey, s.EndKey, s.PauseKey} {
		watchedKeys[i].Store(uint32(code))
	}
	captureKey.Store(a.binding != "")
}

// keySlotsLocked maps each bindable action to its setting.
func (a *App) keySlotsLocked() map[string]*int {
	s := &a.settings
	return map[string]*int{"left": &s.LeftKey, "right": &s.RightKey, "start": &s.StartKey, "end": &s.EndKey, "pause": &s.PauseKey}
}

// bindLocked assigns the pressed key to the action being rebound. Escape
// cancels. Taking the other strafe key swaps the two, and the start and end
// keys may be the same; a key held by any other action is refused, and the
// binding keeps waiting.
func (a *App) bindLocked(code int) {
	if code != 0x01 {
		slots := a.keySlotsLocked()
		target := slots[a.binding]
		partner := map[string]string{"left": "right", "right": "left", "start": "end", "end": "start"}[a.binding]
		for name, slot := range slots {
			if *slot == code && name != a.binding && name != partner {
				return
			}
		}
		if other := slots[partner]; other != nil && *other == code && partner != "start" && partner != "end" {
			*other = *target
		}
		*target = code
		a.holdLocked(code) // its auto-repeat must not fire the new hotkey
		a.saveLocked()
	}
	a.binding = ""
	a.leftDown, a.rightDown = false, false
	a.syncKeysLocked()
}

// BindKey waits for the next key press and binds it to action: a strafe key
// ("left", "right") or a practice key ("start", "end", "pause"). Calling it
// again for the same action cancels.
func (a *App) BindKey(side string) Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.editing && a.keySlotsLocked()[side] != nil && a.binding != side {
		a.binding = side
	} else {
		a.binding = ""
	}
	a.syncKeysLocked()
	a.emitLocked()
	return a.snapshotLocked()
}
func (a *App) cancelLocked() {
	if a.running {
		a.finishLocked(a.elapsed)
	}
	if a.running || a.preview {
		a.silence()
	}
	a.running = false
	a.preview = false
	a.elapsed = 0
}
func (a *App) snapshotLocked() Snapshot {
	phases := a.patternLocked()
	idx, dir, _ := pattern.At(phases, a.elapsed)
	modeLabel := "EXPECTED STRAFE"
	if a.mode.ID != "default" {
		modeLabel = strings.ToUpper(a.mode.Name)
	}
	player, playerEnd, score := a.track, a.elapsed, (*pattern.Score)(nil)
	if !a.running {
		player, playerEnd = nil, 0
		if a.last != nil {
			player, playerEnd, score = a.last.track, a.last.endMS, &a.last.score
		}
	}
	return Snapshot{Player: append([]pattern.Segment{}, player...), PlayerEndMS: playerEnd, Score: score, LeftKey: keyName(a.settings.LeftKey), RightKey: keyName(a.settings.RightKey), StartKey: keyName(a.settings.StartKey), EndKey: keyName(a.settings.EndKey), PauseKey: keyName(a.settings.PauseKey), Binding: a.binding, revision: a.revision,
		Settings: a.settings, Phases: phases, Weapon: a.weapon.Name, Mode: modeLabel, Editing: a.editing, Armed: a.armed, Focused: a.focused, InputReady: a.inputReady, Running: a.running, Preview: a.preview, Moving: a.moving, Shown: a.shown, Held: a.held, ElapsedMS: a.elapsed, TotalMS: pattern.TotalMS(phases), Phase: idx, Direction: dir, RunID: a.runID, Error: a.err}
}

// overlayModeLocked is how the native overlay should be on screen right now.
func (a *App) overlayModeLocked() int {
	switch {
	case !a.editing:
		return overlayPractice
	case a.minimised:
		return overlayHidden
	case a.moving:
		return overlayMove
	case a.shown:
		return overlayPreview
	}
	return overlayHidden
}

// minimiseLocked follows the settings window into and out of the taskbar. While
// editing, the overlay is only a preview, so it goes away with the window.
func (a *App) minimiseLocked(minimised bool) {
	if minimised == a.minimised {
		return
	}
	a.minimised = minimised
	if !a.editing {
		return
	}
	if minimised {
		a.moving = false
		a.cancelLocked()
	}
	if err := a.applyOverlayLocked(); err != nil {
		a.err = err.Error()
	}
}
func (a *App) applyOverlayLocked() error {
	return setOverlayMode(a.overlayModeLocked(), a.settings)
}
func (a *App) emitLocked() {
	if a.overlayModeLocked() != overlayHidden {
		if err := renderPractice(a.snapshotLocked()); err != nil {
			a.err = err.Error()
			a.cancelLocked()
			a.moving, a.shown = false, false
			if restoreErr := setOverlayMode(overlayHidden, a.settings); restoreErr == nil {
				a.editing = true
			} else {
				a.err += "; " + restoreErr.Error()
			}
		} else if !a.editing {
			// The WebView is hidden in practice. Keep its bridge quiet; the native
			// bitmap follows the Go clock even if background web rendering sleeps.
			return
		}
	}
	if a.ctx != nil { // nil before startup and in tests
		wr.EventsEmit(a.ctx, "state", a.snapshotLocked())
	}
}
func (a *App) GetState() Snapshot { a.mu.Lock(); defer a.mu.Unlock(); return a.snapshotLocked() }
func (a *App) toggleLocked() {
	if !a.inputReady && a.editing {
		return
	}
	a.binding = ""
	a.syncKeysLocked()
	moving, minimised := a.moving, a.minimised
	a.editing, a.moving = !a.editing, false
	if a.editing {
		// Leaving practice always brings the editor back, even from the taskbar.
		restoreMainWindow()
		a.minimised = false
	}
	if err := a.applyOverlayLocked(); err != nil {
		a.err = err.Error()
		a.editing, a.moving, a.minimised = !a.editing, moving, minimised
		return
	}
	a.cancelLocked()
	if !a.editing {
		a.captureGeometryLocked()
	}
}
func (a *App) ToggleMode() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.toggleLocked()
	a.emitLocked()
	return a.snapshotLocked()
}

// ToggleMove makes the overlay a draggable window so it can be positioned.
func (a *App) ToggleMove() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.editing {
		a.moving = !a.moving
		if err := a.applyOverlayLocked(); err != nil {
			a.err = err.Error()
			a.moving = !a.moving
		}
	}
	a.emitLocked()
	return a.snapshotLocked()
}

// ToggleOverlay shows or hides the overlay while editing. Practice mode always
// shows it.
func (a *App) ToggleOverlay() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.editing {
		shown, moving := a.shown, a.moving
		a.shown = !a.shown
		if !a.shown {
			a.moving = false
			a.cancelLocked() // a preview nobody can see
		}
		if err := a.applyOverlayLocked(); err != nil {
			a.err = err.Error()
			a.shown, a.moving = shown, moving
		}
	}
	a.emitLocked()
	return a.snapshotLocked()
}

// CenterOverlay puts the overlay back on the crosshair.
func (a *App) CenterOverlay() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.settings.OffsetX, a.settings.OffsetY = 0, 0
	a.saveLocked()
	a.replaceOverlayLocked()
	a.emitLocked()
	return a.snapshotLocked()
}
func (a *App) replaceOverlayLocked() {
	if a.overlayModeLocked() != overlayHidden {
		if err := a.applyOverlayLocked(); err != nil {
			a.err = err.Error()
		}
	}
}
func (a *App) saveLocked() {
	if err := writeSettings(a.settings); err != nil {
		a.err = "Settings could not be saved: " + err.Error()
	}
}
func (a *App) Preview() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.editing {
		// The preview plays on the overlay, so bring it back if it was hidden.
		if !a.shown {
			a.shown = true
			if err := a.applyOverlayLocked(); err != nil {
				a.err = err.Error()
				a.shown = false
			}
		}
		a.startLocked(time.Now(), true)
	}
	a.emitLocked()
	return a.snapshotLocked()
}
func (a *App) StopPreview() { a.mu.Lock(); defer a.mu.Unlock(); a.cancelLocked(); a.emitLocked() }
func (a *App) UpdateSettings(s Settings) (Snapshot, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.editing {
		return a.snapshotLocked(), fmt.Errorf("leave practice mode before editing")
	}
	a.cancelLocked()
	s.X = a.settings.X
	s.Y = a.settings.Y
	s.Positioned = a.settings.Positioned
	s.OffsetX = a.settings.OffsetX
	s.OffsetY = a.settings.OffsetY
	s.LeftKey = a.settings.LeftKey
	s.RightKey = a.settings.RightKey
	s.StartKey = a.settings.StartKey
	s.EndKey = a.settings.EndKey
	s.PauseKey = a.settings.PauseKey
	s.Width = a.settings.Width
	s.Height = a.settings.Height
	s = s.normalized()
	// A lead still at its style's default follows a change of style.
	if s.VoiceStyle != a.settings.VoiceStyle && s.VoiceLeadMS == voiceLeads[a.settings.VoiceStyle] {
		s.VoiceLeadMS = voiceLeads[s.VoiceStyle]
	}
	if s.WeaponID != a.settings.WeaponID || s.ModeID != a.settings.ModeID {
		a.last = nil // the last spray was scored against another pattern
	}
	a.settings = s
	err := writeSettings(a.settings)
	if err != nil {
		a.err = "Settings could not be saved: " + err.Error()
	}
	a.replaceOverlayLocked() // its size follows the arrow and timeline settings
	a.emitLocked()
	return a.snapshotLocked(), err
}
func (a *App) captureGeometryLocked() {
	// A minimised window reports a parked position and size; keep the last real one.
	if wr.WindowIsMinimised(a.ctx) {
		return
	}
	a.settings.X, a.settings.Y = wr.WindowGetPosition(a.ctx)
	a.settings.Width, a.settings.Height = wr.WindowGetSize(a.ctx)
	a.settings.Positioned = true
	if err := writeSettings(a.settings); err != nil {
		a.err = "Settings could not be saved: " + err.Error()
	}
}
func (a *App) SavePosition()                        { a.mu.Lock(); defer a.mu.Unlock(); a.captureGeometryLocked() }
func (a *App) Quit()                                { a.SavePosition(); wr.Quit(a.ctx) }
func (a *App) beforeClose(ctx context.Context) bool { a.SavePosition(); return false }
