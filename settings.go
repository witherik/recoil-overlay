package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"recoil-overlay/internal/pattern"
)

type Settings struct {
	WeaponID       string `json:"weaponId"`
	ModeID         string `json:"modeId"`
	Gap            int    `json:"gap"`
	ArrowSize      int    `json:"arrowSize"`
	Opacity        int    `json:"opacity"`
	Theme          string `json:"theme"` // colour scheme, see palettes
	Arrows         bool   `json:"arrows"`
	Timeline       bool   `json:"timeline"`
	TimelineIdle   bool   `json:"timelineIdle"` // hide the timeline while left-click is held
	OffsetX        int    `json:"offsetX"`
	OffsetY        int    `json:"offsetY"`
	TimelineOffset int    `json:"timelineOffset"`
	TimelineWidth  int    `json:"timelineWidth"`
	LeftKey        int    `json:"leftKey"` // keyboard scan codes; 0xE000 marks extended keys
	RightKey       int    `json:"rightKey"`
	StartKey       int    `json:"startKey"` // enters practice; may equal EndKey, making one toggle
	EndKey         int    `json:"endKey"`
	Voice          bool   `json:"voice"`
	VoiceStyle     string `json:"voiceStyle"` // "fast", "natural" or "tones"
	VoiceStart     bool   `json:"voiceStart"` // also announce the first strafe of a spray
	VoiceLeadMS    int    `json:"voiceLeadMs"`
	X              int    `json:"x"`
	Y              int    `json:"y"`
	Positioned     bool   `json:"positioned"`
}

// voiceLeads is the default lead of each voice style. A word needs time to be
// heard before its switch: the fast clips are over within about 75 ms, so
// with a 60 ms lead each word is all but finished as its direction change
// arrives. A tone is understood at once, so it sounds on the switch itself.
var voiceLeads = map[string]int{"fast": 60, "natural": 125, "tones": 0}

func defaultSettings() Settings {
	return Settings{
		WeaponID: "r301", ModeID: "default",
		Gap: 100, ArrowSize: 42, Opacity: 90, Theme: "green",
		Arrows: true, Timeline: true, TimelineOffset: 160, TimelineWidth: 420,
		LeftKey: 0x1E, RightKey: 0x20, // A and D
		StartKey: 0x42, EndKey: 0x42, // F8
		Voice: true, VoiceStyle: "fast", VoiceLeadMS: voiceLeads["fast"],
	}
}

func clamp(v, low, high int) int { return min(max(v, low), high) }

// normalized returns s with every value made valid: numbers are clamped to
// their ranges, and unknown names and unusable keys fall back to the defaults.
func (s Settings) normalized() Settings {
	d := defaultSettings()
	weapon, mode := pattern.Find(s.WeaponID, s.ModeID)
	s.WeaponID, s.ModeID = weapon.ID, mode.ID
	s.Gap = clamp(s.Gap, 40, 300)
	s.ArrowSize = clamp(s.ArrowSize, 24, 72)
	s.Opacity = clamp(s.Opacity, 20, 100)
	if _, known := palettes[s.Theme]; !known {
		s.Theme = d.Theme
	}
	s.TimelineOffset = clamp(s.TimelineOffset, -800, 800)
	s.TimelineWidth = clamp(s.TimelineWidth, 280, 800)
	s.OffsetX = clamp(s.OffsetX, -10000, 10000)
	s.OffsetY = clamp(s.OffsetY, -10000, 10000)
	if s.LeftKey <= 0 || s.RightKey <= 0 || s.LeftKey == s.RightKey {
		s.LeftKey, s.RightKey = d.LeftKey, d.RightKey
	}
	if s.StartKey <= 0 || s.EndKey <= 0 {
		s.StartKey, s.EndKey = d.StartKey, d.EndKey
	}
	if _, known := voiceLeads[s.VoiceStyle]; !known {
		s.VoiceStyle = d.VoiceStyle
	}
	s.VoiceLeadMS = clamp(s.VoiceLeadMS, 0, 350)
	return s
}

func settingsPath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	return filepath.Join(base, "RecoilPractice", "settings.json")
}

// readSettings loads the saved settings. A missing or unreadable file gives
// the defaults.
func readSettings() Settings {
	s := defaultSettings()
	b, err := os.ReadFile(settingsPath())
	if err == nil {
		if json.Unmarshal(b, &s) != nil {
			return defaultSettings()
		}
	}
	return s.normalized()
}

// writeSettings replaces the settings file in one step, so a crash cannot
// leave half of one behind.
func writeSettings(s Settings) error {
	path := settingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(path+".tmp", b, 0600); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}
