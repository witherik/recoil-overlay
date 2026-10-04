package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Settings struct {
	Gap            int  `json:"gap"`
	ArrowSize      int  `json:"arrowSize"`
	Opacity        int  `json:"opacity"`
	Arrows         bool `json:"arrows"`
	Timeline       bool `json:"timeline"`
	TimelineIdle   bool `json:"timelineIdle"` // hide the timeline while left-click is held
	OffsetX        int  `json:"offsetX"`
	OffsetY        int  `json:"offsetY"`
	TimelineOffset int  `json:"timelineOffset"`
	Voice          bool `json:"voice"`
	VoiceLeadMS    int  `json:"voiceLeadMs"`
	X              int  `json:"x"`
	Y              int  `json:"y"`
	Width          int  `json:"width"`
	Height         int  `json:"height"`
	Positioned     bool `json:"positioned"`
}

func defaultSettings() Settings {
	return Settings{Gap: 100, ArrowSize: 42, Opacity: 90, Arrows: true, Timeline: true, TimelineOffset: 32, Voice: true, VoiceLeadMS: 150, Width: 620, Height: 600}
}
func clamp(v, low, high int) int {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}
func (s Settings) normalized() Settings {
	s.Gap = clamp(s.Gap, 40, 300)
	s.ArrowSize = clamp(s.ArrowSize, 24, 72)
	s.Opacity = clamp(s.Opacity, 20, 100)
	s.TimelineOffset = clamp(s.TimelineOffset, 12, 100)
	s.OffsetX = clamp(s.OffsetX, -10000, 10000)
	s.OffsetY = clamp(s.OffsetY, -10000, 10000)
	s.VoiceLeadMS = clamp(s.VoiceLeadMS, 0, 350)
	s.Width = clamp(s.Width, 520, 1400)
	s.Height = clamp(s.Height, 560, 1000)
	return s
}
func settingsPath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	return filepath.Join(base, "RecoilPractice", "settings.json")
}
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
