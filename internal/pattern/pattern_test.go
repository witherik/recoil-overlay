package pattern

import "testing"

func TestR301ReturnsFreshPattern(t *testing.T) {
	want := []Phase{
		{Direction: "right", DurationMS: 800},
		{Direction: "left", DurationMS: 530},
		{Direction: "right", DurationMS: 880},
	}
	first := R301()
	if len(first) != len(want) {
		t.Fatalf("R301() length = %d, want %d", len(first), len(want))
	}
	for i := range want {
		if first[i] != want[i] {
			t.Errorf("R301()[%d] = %#v, want %#v", i, first[i], want[i])
		}
	}
	first[0].Direction = "changed"
	if got := R301()[0].Direction; got != "right" {
		t.Errorf("mutating one result changed a later result: direction = %q", got)
	}
}

func TestTotalMS(t *testing.T) {
	if got := TotalMS(R301()); got != 2210 {
		t.Errorf("TotalMS(R301()) = %d, want 2210", got)
	}
	if got := TotalMS(nil); got != 0 {
		t.Errorf("TotalMS(nil) = %d, want 0", got)
	}
}

func TestAtBoundaries(t *testing.T) {
	phases := R301()
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
	phases := R301()
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
