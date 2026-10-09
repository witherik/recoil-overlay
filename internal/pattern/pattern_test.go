package pattern

import "testing"

// r301 is the pattern most tests use: right 800, left 530, right 880.
func r301() []Phase {
	_, mode := Find("r301", "default")
	return mode.Phases
}

func TestAtBoundaries(t *testing.T) {
	phases := r301()
	tests := []struct {
		elapsed int64
		index   int
		dir     string
		done    bool
	}{
		{-1, 0, "right", false},
		{0, 0, "right", false},
		{799, 0, "right", false},
		{800, 1, "left", false},
		{1329, 1, "left", false},
		{1330, 2, "right", false},
		{2209, 2, "right", false},
		{2210, -1, "", true},
		{3000, -1, "", true},
	}
	for _, tt := range tests {
		index, direction, done := At(phases, tt.elapsed)
		if index != tt.index || direction != tt.dir || done != tt.done {
			t.Errorf("At(phases, %d) = (%d, %q, %t), want (%d, %q, %t)", tt.elapsed, index, direction, done, tt.index, tt.dir, tt.done)
		}
	}
	if index, direction, done := At(nil, 0); index != -1 || direction != "" || !done {
		t.Errorf("At(nil, 0) = (%d, %q, %t), want (-1, %q, true)", index, direction, done, "")
	}
}

func TestCues(t *testing.T) {
	phases := r301()
	got := Cues(phases, 100)
	want := []Cue{
		{AtMS: 0, Direction: "right"},
		{AtMS: 700, Direction: "left"},
		{AtMS: 1230, Direction: "right"},
	}
	if len(got) != len(want) {
		t.Fatalf("Cues() length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Cues()[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}

	clamped := Cues(phases, 2000)
	for i, cue := range clamped {
		if cue.AtMS != 0 {
			t.Errorf("Cues(..., 2000)[%d].AtMS = %d, want 0", i, cue.AtMS)
		}
	}
	if cues := Cues(nil, 100); len(cues) != 0 {
		t.Errorf("Cues(nil, 100) = %#v, want empty", cues)
	}
}
