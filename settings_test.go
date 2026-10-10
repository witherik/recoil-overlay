package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsRoundTripAndCorruption(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	s := defaultSettings()
	s.Gap = 180
	s.VoiceLeadMS = 210
	s.X = -500
	s.Positioned = true
	if err := writeSettings(s); err != nil {
		t.Fatal(err)
	}
	if got := readSettings(); got != s {
		t.Fatalf("got %+v want %+v", got, s)
	}
	s.Gap = 200
	if err := writeSettings(s); err != nil {
		t.Fatal("replace existing settings:", err)
	}
	if readSettings().Gap != 200 {
		t.Fatal("replacement was not saved")
	}
	if err := os.WriteFile(filepath.Clean(settingsPath()), []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if readSettings() != defaultSettings() {
		t.Fatal("corrupt config should use defaults")
	}
	// A file from before the two practice keys became one.
	if err := os.WriteFile(settingsPath(), []byte(`{"startKey": 65, "endKey": 66}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got := readSettings(); got.PracticeKey != 65 || got.ShootKey != leftMouse {
		t.Fatalf("old start key should be kept: %+v", got)
	}
}
