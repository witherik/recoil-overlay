package main

import (
	"context"
	"fmt"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
	"recoil-overlay/internal/pattern"
	"sync"
	"time"
)

const windowTitle = "Recoil Practice"

type Snapshot struct {
	Settings   Settings        `json:"settings"`
	Phases     []pattern.Phase `json:"phases"`
	Editing    bool            `json:"editing"`
	Armed      bool            `json:"armed"`
	Focused    bool            `json:"focused"`
	InputReady bool            `json:"inputReady"`
	Running    bool            `json:"running"`
	Preview    bool            `json:"preview"`
	Moving     bool            `json:"moving"`
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
	leftDown, rightDown                                         bool
	binding                                                     string // "left" or "right" while waiting for a key
	track                                                       []pattern.Segment
	last                                                        *review
	revision                                                    uint64
	started                                                     time.Time
	elapsed                                                     int64
	runID                                                       uint64
	nextCue                                                     int
	cues                                                        []pattern.Cue
	err                                                         string
	speak                                                       func(string)
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
			a.emitLocked()
			a.mu.Unlock()
		}
	}
}
func (a *App) handleInputLocked(e inputEvent) {
	switch e.Kind {
	case "toggle":
		a.toggleLocked()
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
				a.editing = true
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
	if a.elapsed >= int64(pattern.TotalMS(pattern.R301())) {
		a.elapsed = int64(pattern.TotalMS(pattern.R301()))
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
	// Neutral phases (a charge-up before the first strafe) have no voice line.
	if due >= 0 && a.settings.Voice && a.cues[due].Direction != "" {
		a.speak(a.cues[due].Direction)
	}
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
	a.cues = pattern.Cues(pattern.R301(), a.settings.VoiceLeadMS)
	a.nextCue = 0
	if a.settings.Voice {
		a.speak("right")
	}
	a.nextCue = 1
}

// clockLocked converts a capture time to milliseconds into the current spray.
func (a *App) clockLocked(at time.Time) int64 {
	return min(max(at.Sub(a.started).Milliseconds(), 0), int64(pattern.TotalMS(pattern.R301())))
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
		a.last = &review{track: a.track, endMS: endMS, score: pattern.Evaluate(pattern.R301(), a.track, endMS)}
	}
	a.track = nil
	a.revision++
}
func (a *App) syncKeysLocked() {
	strafeLeftCode.Store(uint32(a.settings.LeftKey))
	strafeRightCode.Store(uint32(a.settings.RightKey))
	captureKey.Store(a.binding != "")
}

// bindLocked assigns the pressed key to the side being rebound. Escape cancels;
// taking the other side's key swaps the two.
func (a *App) bindLocked(code int) {
	if code != 0x01 {
		left, right := &a.settings.LeftKey, &a.settings.RightKey
		if a.binding == "right" {
			left, right = right, left
		}
		if code == *right {
			*right = *left
		}
		*left = code
		a.saveLocked()
	}
	a.binding = ""
	a.leftDown, a.rightDown = false, false
	a.syncKeysLocked()
}

// BindKey waits for the next key press and makes it the strafe key for side
// ("left" or "right"). Calling it again for the same side cancels.
func (a *App) BindKey(side string) Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.editing && (side == "left" || side == "right") && a.binding != side {
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
	idx, dir, _ := pattern.At(pattern.R301(), a.elapsed)
	player, playerEnd, score := a.track, a.elapsed, (*pattern.Score)(nil)
	if !a.running {
		player, playerEnd = nil, 0
		if a.last != nil {
			player, playerEnd, score = a.last.track, a.last.endMS, &a.last.score
		}
	}
	return Snapshot{Player: append([]pattern.Segment{}, player...), PlayerEndMS: playerEnd, Score: score, LeftKey: keyName(a.settings.LeftKey), RightKey: keyName(a.settings.RightKey), Binding: a.binding, revision: a.revision,
		Settings: a.settings, Phases: pattern.R301(), Editing: a.editing, Armed: a.armed, Focused: a.focused, InputReady: a.inputReady, Running: a.running, Preview: a.preview, Moving: a.moving, Held: a.held, ElapsedMS: a.elapsed, TotalMS: pattern.TotalMS(pattern.R301()), Phase: idx, Direction: dir, RunID: a.runID, Error: a.err}
}
func (a *App) emitLocked() {
	if !a.editing || a.moving {
		if err := renderPractice(a.snapshotLocked()); err != nil {
			a.err = err.Error()
			a.cancelLocked()
			if restoreErr := setOverlayMode(overlayHidden, a.settings); restoreErr == nil {
				a.editing, a.moving = true, false
			} else {
				a.err += "; " + restoreErr.Error()
			}
		} else if !a.editing {
			// The WebView is hidden in practice. Keep its bridge quiet; the native
			// bitmap follows the Go clock even if background web rendering sleeps.
			return
		}
	}
	wr.EventsEmit(a.ctx, "state", a.snapshotLocked())
}
func (a *App) GetState() Snapshot { a.mu.Lock(); defer a.mu.Unlock(); return a.snapshotLocked() }
func (a *App) toggleLocked() {
	if !a.inputReady && a.editing {
		return
	}
	editing := !a.editing
	a.binding = ""
	a.syncKeysLocked()
	mode := overlayPractice
	if editing {
		mode = overlayHidden
	}
	if err := setOverlayMode(mode, a.settings); err != nil {
		a.err = err.Error()
		return
	}
	a.cancelLocked()
	a.editing, a.moving = editing, false
	if !editing {
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

// ToggleMove shows the overlay as a draggable window so it can be positioned.
func (a *App) ToggleMove() Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.editing {
		mode := overlayMove
		if a.moving {
			mode = overlayHidden
		}
		if err := setOverlayMode(mode, a.settings); err != nil {
			a.err = err.Error()
		} else {
			a.moving = !a.moving
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
	if a.moving {
		if err := setOverlayMode(overlayMove, a.settings); err != nil {
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
		return a.snapshotLocked(), fmt.Errorf("unlock the overlay with F8 before editing")
	}
	a.cancelLocked()
	s.X = a.settings.X
	s.Y = a.settings.Y
	s.Positioned = a.settings.Positioned
	s.OffsetX = a.settings.OffsetX
	s.OffsetY = a.settings.OffsetY
	s.LeftKey = a.settings.LeftKey
	s.RightKey = a.settings.RightKey
	s.Width = a.settings.Width
	s.Height = a.settings.Height
	a.settings = s.normalized()
	err := writeSettings(a.settings)
	if err != nil {
		a.err = "Settings could not be saved: " + err.Error()
	}
	a.replaceOverlayLocked() // its size follows the arrow and timeline settings
	a.emitLocked()
	return a.snapshotLocked(), err
}
func (a *App) captureGeometryLocked() {
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
