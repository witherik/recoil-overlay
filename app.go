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
	Held       bool            `json:"held"`
	ElapsedMS  int64           `json:"elapsedMs"`
	TotalMS    int             `json:"totalMs"`
	Phase      int             `json:"phase"`
	Direction  string          `json:"direction"`
	RunID      uint64          `json:"runId"`
	Error      string          `json:"error"`
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
	in, err := startNativeInput(a.events)
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
			a.cancelLocked()
		}
	case "error":
		a.err = e.Detail
		a.inputReady = false
		a.cancelLocked()
		// A failed input thread must never strand the user behind a click-through window.
		if !a.editing {
			if err := setPracticeWindow(false, false); err == nil {
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
		a.running = false
		a.silence()
		return
	}
	// If the process was stalled, announce only the latest due cue, never a backlog.
	due := -1
	for a.nextCue < len(a.cues) && a.elapsed >= a.cues[a.nextCue].AtMS {
		due = a.nextCue
		a.nextCue++
	}
	if due >= 0 && a.settings.Voice {
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
	a.cues = pattern.Cues(pattern.R301(), a.settings.VoiceLeadMS)
	a.nextCue = 0
	if a.settings.Voice {
		a.speak("right")
	}
	a.nextCue = 1
}
func (a *App) cancelLocked() {
	if a.running || a.preview {
		a.silence()
	}
	a.running = false
	a.preview = false
	a.elapsed = 0
}
func (a *App) snapshotLocked() Snapshot {
	idx, dir, _ := pattern.At(pattern.R301(), a.elapsed)
	return Snapshot{Settings: a.settings, Phases: pattern.R301(), Editing: a.editing, Armed: a.armed, Focused: a.focused, InputReady: a.inputReady, Running: a.running, Preview: a.preview, Held: a.held, ElapsedMS: a.elapsed, TotalMS: pattern.TotalMS(pattern.R301()), Phase: idx, Direction: dir, RunID: a.runID, Error: a.err}
}
func (a *App) emitLocked() {
	if !a.editing {
		if err := renderPractice(a.snapshotLocked()); err != nil {
			a.err = err.Error()
			a.cancelLocked()
			if restoreErr := setPracticeWindow(false, false); restoreErr == nil {
				a.editing = true
			} else {
				a.err += "; " + restoreErr.Error()
			}
		} else {
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
	if err := setPracticeWindow(!editing, a.settings.AutoCenter); err != nil {
		a.err = err.Error()
		return
	}
	a.cancelLocked()
	a.editing = editing
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
	s.Width = a.settings.Width
	s.Height = a.settings.Height
	a.settings = s.normalized()
	err := writeSettings(a.settings)
	if err != nil {
		a.err = "Settings could not be saved: " + err.Error()
	}
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
func (a *App) CenterWindow()                        { wr.WindowCenter(a.ctx); a.SavePosition() }
func (a *App) Quit()                                { a.SavePosition(); wr.Quit(a.ctx) }
func (a *App) beforeClose(ctx context.Context) bool { a.SavePosition(); return false }
