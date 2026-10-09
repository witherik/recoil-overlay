package main

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	wr "github.com/wailsapp/wails/v2/pkg/runtime"

	"recoil-overlay/internal/pattern"
)

const windowTitle = "Recoil Practice"

// Snapshot is the whole state of the app as the overlay and the settings
// window show it.
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
	Error      string          `json:"error"` // shown until dismissed, see DismissError
	Seq        uint64          `json:"seq"`   // grows with every change, so a late copy of an old state can be told apart

	// The player's own strafes: live while a spray runs, then the last spray.
	Player      []pattern.Segment `json:"player"`
	PlayerEndMS int64             `json:"playerEndMs"`
	Score       *pattern.Score    `json:"score"`

	// The bound keys by name, and the action waiting for a new one.
	LeftKey  string `json:"leftKey"`
	RightKey string `json:"rightKey"`
	StartKey string `json:"startKey"`
	EndKey   string `json:"endKey"`
	PauseKey string `json:"pauseKey"`
	Binding  string `json:"binding"`

	revision uint64
}

// A spray shorter than this is a tap, and does not replace the last review.
const minReviewMS = 250

// A review is a finished spray, kept on the timeline until the next one.
type review struct {
	track []pattern.Segment
	endMS int64
	score pattern.Score
}

// A view is what decides how the overlay and the settings window are on
// screen, see overlayModeLocked.
type view struct {
	editing   bool // the settings window is up; otherwise practice
	moving    bool // the overlay is being dragged into place
	shown     bool // show the overlay while editing too
	minimised bool // the settings window is in the taskbar
}

type App struct {
	mu       sync.Mutex
	ctx      context.Context
	settings Settings
	err      string

	// The input thread, and the loop that handles its events.
	input  *nativeInput
	events chan inputEvent
	stop   chan struct{}
	done   chan struct{}

	view
	armed, focused, inputReady bool
	held                       bool         // left mouse button
	leftDown, rightDown        bool         // strafe keys
	binding                    string       // the action waiting for a key, see keySlotsLocked
	hotDown                    map[int]bool // practice keys still down, see hotkeyLocked

	// The spray in progress.
	running, preview bool
	started          time.Time
	elapsed          int64
	runID            uint64
	cues             []pattern.Cue
	nextCue          int
	firstCue         int               // cue of the spray's first strafe
	track            []pattern.Segment // the player's strafes so far
	last             *review
	revision         uint64 // counts changes to track and last, so the overlay redraws

	// The state the settings window was last sent, and how many it has been sent.
	sent *Snapshot
	seq  uint64

	speak   func(style, direction string)
	silence func()
}

func NewApp() *App {
	return &App{
		settings: readSettings(),
		view:     view{editing: true},
		armed:    true,
		events:   make(chan inputEvent, 256),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
		speak:    playVoice,
		silence:  stopVoice,
	}
}

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

func (a *App) ready(ctx context.Context) {
	a.mu.Lock()
	s := a.settings
	a.syncKeysLocked()
	a.mu.Unlock()
	if s.Positioned {
		restorePosition(ctx, s.X, s.Y)
	}
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

func (a *App) beforeClose(ctx context.Context) bool {
	a.savePosition()
	return false
}

// loop is the app's clock: it applies input as it arrives and, sixty times a
// second, advances the spray and redraws. A tick on which nothing has changed
// costs a few microseconds: nothing is drawn or sent.
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
		if !a.editing {
			a.retreatLocked()
		}
	}
}

// The spray: its clock, its voice cues and the player's strafes.

// startLocked begins a spray at the moment of the click, or a preview of one.
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
	a.firstCue = 0
	for a.firstCue < len(a.cues)-1 && a.cues[a.firstCue].Direction == "" {
		a.firstCue++
	}
	// The first cue cannot lead: the click is not predictable.
	a.announceLocked(0)
	a.nextCue = 1
}

// advanceLocked moves a running spray on to now, announcing the cues that have
// come due and ending it after its last phase. Only a preview runs without
// Apex in the foreground.
func (a *App) advanceLocked(now time.Time, focused bool) {
	a.focused = focused
	if !focused && !a.preview {
		a.cancelLocked()
	}
	if !a.running {
		return
	}
	a.elapsed = a.clockLocked(now)
	if a.elapsed == int64(pattern.TotalMS(a.patternLocked())) {
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
	if due >= 0 {
		a.announceLocked(due)
	}
}

// announceLocked speaks a cue. Neutral phases (a charge-up before the first
// strafe) have no voice line, and the opening strafe is announced only if
// asked for.
func (a *App) announceLocked(cue int) {
	direction := a.cues[cue].Direction
	if a.settings.Voice && direction != "" && (cue != a.firstCue || a.settings.VoiceStart) {
		a.speak(a.settings.VoiceStyle, direction)
	}
}

// cancelLocked ends a spray or preview where it stands and resets the clock.
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

// finishLocked closes the spray at endMS and keeps it for review.
func (a *App) finishLocked(endMS int64) {
	a.strafeLocked(endMS)
	if endMS >= minReviewMS {
		a.last = &review{track: a.track, endMS: endMS, score: pattern.Evaluate(a.patternLocked(), a.track, endMS)}
	}
	a.track = nil
	a.revision++
}

// patternLocked returns the phases of the selected weapon and mode.
func (a *App) patternLocked() []pattern.Phase {
	_, mode := pattern.Find(a.settings.WeaponID, a.settings.ModeID)
	return mode.Phases
}

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

// The keys: the practice hotkeys, and rebinding.

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
		a.pauseLocked()
	case a.editing && code == s.StartKey, !a.editing && code == s.EndKey:
		a.toggleLocked()
	}
}

// holdLocked marks a key as down, so its auto-repeat is not taken for a press.
func (a *App) holdLocked(code int) {
	if a.hotDown == nil {
		a.hotDown = map[int]bool{}
	}
	a.hotDown[code] = true
}

// pauseLocked disables or enables practice input.
func (a *App) pauseLocked() {
	a.armed = !a.armed
	a.cancelLocked()
}

// syncKeysLocked tells the input thread which keys to forward.
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

const escapeKey = 0x01

// bindLocked assigns the pressed key to the action being rebound. Escape
// cancels. Taking the other strafe key swaps the two, and the start and end
// keys may be the same; a key held by any other action is refused, and the
// binding keeps waiting.
func (a *App) bindLocked(code int) {
	if code != escapeKey {
		slots := a.keySlotsLocked()
		target := slots[a.binding]
		partner := map[string]string{"left": "right", "right": "left", "start": "end", "end": "start"}[a.binding]
		for name, slot := range slots {
			if *slot == code && name != a.binding && name != partner {
				return
			}
		}
		if strafe := a.binding == "left" || a.binding == "right"; strafe && *slots[partner] == code {
			*slots[partner] = *target
		}
		*target = code
		a.holdLocked(code) // its auto-repeat must not fire the new hotkey
		a.saveLocked()
	}
	a.binding = ""
	a.leftDown, a.rightDown = false, false
	a.syncKeysLocked()
}

// The view: where the overlay and the settings window are.

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

func (a *App) applyOverlayLocked() error {
	return setOverlayMode(a.overlayModeLocked(), a.settings)
}

// setViewLocked moves the windows to a new view. If Windows refuses, the old
// view stays and the error is shown.
func (a *App) setViewLocked(next view) bool {
	old := a.view
	a.view = next
	if err := a.applyOverlayLocked(); err != nil {
		a.err = err.Error()
		a.view = old
		return false
	}
	return true
}

// replaceOverlayLocked puts a visible overlay where the settings now say.
func (a *App) replaceOverlayLocked() {
	if a.overlayModeLocked() != overlayHidden {
		if err := a.applyOverlayLocked(); err != nil {
			a.err = err.Error()
		}
	}
}

// toggleLocked switches between editing and practice. Practice needs input.
func (a *App) toggleLocked() {
	if a.editing && !a.inputReady {
		return
	}
	a.binding = ""
	a.syncKeysLocked()
	next := a.view
	next.editing, next.moving = !a.editing, false
	if a.editing {
		a.captureGeometryLocked() // before practice minimises the window
	} else {
		// Leaving practice always brings the editor back, even from the taskbar.
		restoreMainWindow()
		next.minimised = false
	}
	if a.setViewLocked(next) {
		a.cancelLocked()
	}
}

// minimiseLocked follows the settings window into and out of the taskbar. While
// editing, the overlay is only a preview, so it goes away with the window.
// Practice keeps the window minimised, so bringing it back from the taskbar
// ends practice.
func (a *App) minimiseLocked(minimised bool) {
	if minimised == a.minimised {
		return
	}
	a.minimised = minimised
	if !a.editing {
		if !minimised {
			a.toggleLocked()
		}
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

// retreatLocked hides the overlay and, from practice, returns to the editor: a
// failure must never strand the user behind a click-through window.
func (a *App) retreatLocked() {
	a.moving, a.shown = false, false
	if err := setOverlayMode(overlayHidden, a.settings); err != nil {
		a.err += "; " + err.Error()
		return
	}
	if !a.editing {
		a.editing, a.minimised = true, false
		restoreMainWindow()
	}
}

func (a *App) snapshotLocked() Snapshot {
	weapon, mode := pattern.Find(a.settings.WeaponID, a.settings.ModeID)
	phase, direction, _ := pattern.At(mode.Phases, a.elapsed)
	modeLabel := "EXPECTED STRAFE"
	if mode.ID != "default" {
		modeLabel = strings.ToUpper(mode.Name)
	}
	player, playerEnd, score := a.track, a.elapsed, (*pattern.Score)(nil)
	if !a.running {
		player, playerEnd = nil, 0
		if a.last != nil {
			player, playerEnd, score = a.last.track, a.last.endMS, &a.last.score
		}
	}
	return Snapshot{
		Settings:   a.settings,
		Phases:     mode.Phases,
		Weapon:     weapon.Name,
		Mode:       modeLabel,
		Editing:    a.editing,
		Armed:      a.armed,
		Focused:    a.focused,
		InputReady: a.inputReady,
		Running:    a.running,
		Preview:    a.preview,
		Moving:     a.moving,
		Shown:      a.shown,
		Held:       a.held,
		ElapsedMS:  a.elapsed,
		TotalMS:    pattern.TotalMS(mode.Phases),
		Phase:      phase,
		Direction:  direction,
		RunID:      a.runID,
		Error:      a.err,
		Seq:        a.seq,

		Player:      append([]pattern.Segment{}, player...),
		PlayerEndMS: playerEnd,
		Score:       score,

		LeftKey:  keyName(a.settings.LeftKey),
		RightKey: keyName(a.settings.RightKey),
		StartKey: keyName(a.settings.StartKey),
		EndKey:   keyName(a.settings.EndKey),
		PauseKey: keyName(a.settings.PauseKey),
		Binding:  a.binding,

		revision: a.revision,
	}
}

// emitLocked draws the current state on the overlay and returns it. If it is
// not the state the settings window already has, it is sent there too.
func (a *App) emitLocked() Snapshot {
	s := a.snapshotLocked()
	if a.overlayModeLocked() != overlayHidden {
		if err := renderPractice(s); err != nil {
			a.err = err.Error()
			a.cancelLocked()
			a.retreatLocked()
			s = a.snapshotLocked()
		} else if !a.editing {
			// The WebView is hidden in practice. Keep its bridge quiet; the native
			// bitmap follows the Go clock even if background web rendering sleeps.
			a.sent = nil // so the window is brought up to date when it returns
			return s
		}
	}
	if a.sent != nil && reflect.DeepEqual(s, *a.sent) {
		return s
	}
	a.seq++
	s.Seq = a.seq
	a.sent = &s
	if a.ctx != nil { // nil before startup and in tests
		wr.EventsEmit(a.ctx, "state", s)
	}
	return s
}

// saveLocked writes the settings file. A failure is shown in the window.
func (a *App) saveLocked() error {
	err := writeSettings(a.settings)
	if err != nil {
		a.err = "Settings could not be saved: " + err.Error()
	}
	return err
}

// captureGeometryLocked remembers where the settings window is.
func (a *App) captureGeometryLocked() {
	// A minimised window reports a parked position; keep the last real one.
	if wr.WindowIsMinimised(a.ctx) {
		return
	}
	a.settings.X, a.settings.Y = wr.WindowGetPosition(a.ctx)
	a.settings.Positioned = true
	a.saveLocked()
}

func (a *App) savePosition() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.captureGeometryLocked()
}

// The commands of the settings window. Each returns the state it leaves.

func (a *App) GetState() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.snapshotLocked()
}

// Weapons lists every preset for the settings panel.
func (a *App) Weapons() []pattern.Weapon { return pattern.Weapons() }

func (a *App) ToggleMode() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.toggleLocked()
	return a.emitLocked()
}

// ToggleMove makes the overlay a draggable window so it can be positioned.
func (a *App) ToggleMove() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.editing {
		next := a.view
		next.moving = !a.moving
		a.setViewLocked(next)
	}
	return a.emitLocked()
}

// ToggleOverlay shows or hides the overlay while editing. Practice mode always
// shows it.
func (a *App) ToggleOverlay() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.editing {
		next := a.view
		next.shown = !a.shown
		if !next.shown {
			next.moving = false
			a.cancelLocked() // a preview nobody can see
		}
		a.setViewLocked(next)
	}
	return a.emitLocked()
}

// CenterOverlay puts the overlay back on the crosshair.
func (a *App) CenterOverlay() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.settings.OffsetX, a.settings.OffsetY = 0, 0
	a.saveLocked()
	a.replaceOverlayLocked()
	return a.emitLocked()
}

// Preview runs the pattern on the overlay without Apex.
func (a *App) Preview() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.editing {
		// The preview plays on the overlay, so bring it back if it was hidden.
		if !a.shown {
			next := a.view
			next.shown = true
			a.setViewLocked(next)
		}
		a.startLocked(time.Now(), true)
	}
	return a.emitLocked()
}

func (a *App) StopPreview() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cancelLocked()
	a.emitLocked()
}

// DismissError takes the error message down.
func (a *App) DismissError() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.err = ""
	return a.emitLocked()
}

// BindKey waits for the next key press and binds it to action: a strafe key
// ("left", "right") or a practice key ("start", "end", "pause"). Calling it
// again for the same action cancels.
func (a *App) BindKey(action string) Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.editing && a.keySlotsLocked()[action] != nil && a.binding != action {
		a.binding = action
	} else {
		a.binding = ""
	}
	a.syncKeysLocked()
	return a.emitLocked()
}

// UpdateSettings applies the settings window's edits.
func (a *App) UpdateSettings(s Settings) (Snapshot, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.editing {
		return a.snapshotLocked(), fmt.Errorf("leave practice mode before editing")
	}
	a.cancelLocked()
	// The window position, the overlay offset and the keys are not edited
	// here: each has its own command.
	old := a.settings
	s.X, s.Y, s.Positioned = old.X, old.Y, old.Positioned
	s.OffsetX, s.OffsetY = old.OffsetX, old.OffsetY
	s.LeftKey, s.RightKey, s.StartKey, s.EndKey, s.PauseKey = old.LeftKey, old.RightKey, old.StartKey, old.EndKey, old.PauseKey
	s = s.normalized()
	// A lead still at its style's default follows a change of style.
	if s.VoiceStyle != old.VoiceStyle && s.VoiceLeadMS == voiceLeads[old.VoiceStyle] {
		s.VoiceLeadMS = voiceLeads[s.VoiceStyle]
	}
	if s.WeaponID != old.WeaponID || s.ModeID != old.ModeID {
		a.forgetReviewLocked() // the last spray was scored against another pattern
	}
	a.settings = s
	err := a.saveLocked()
	a.replaceOverlayLocked() // its size follows the arrow and timeline settings
	return a.emitLocked(), err
}

// ResetDefaults puts every setting back to its default, keys and overlay
// position included. The settings window stays where it is.
func (a *App) ResetDefaults() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.editing {
		a.cancelLocked()
		s := defaultSettings()
		s.X, s.Y, s.Positioned = a.settings.X, a.settings.Y, a.settings.Positioned
		a.settings = s
		a.forgetReviewLocked() // the last spray may have been scored against another pattern
		a.binding = ""
		a.syncKeysLocked()
		a.saveLocked()
		a.replaceOverlayLocked()
	}
	return a.emitLocked()
}

// forgetReviewLocked takes the last spray off the timeline.
func (a *App) forgetReviewLocked() {
	a.last = nil
	a.revision++
}

func (a *App) Quit() {
	a.savePosition()
	wr.Quit(a.ctx)
}
