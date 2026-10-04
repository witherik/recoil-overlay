package pattern

import "testing"

func TestEvaluate(t *testing.T) {
	tests := []struct {
		name    string
		player  []Segment
		end     int64
		want    []Change
		average int
		missed  int
	}{
		{
			name:   "perfect",
			player: []Segment{{0, 800, "right"}, {800, 1330, "left"}, {1330, 2210, "right"}},
			end:    2210,
			want:   []Change{{0, "right", 0, false}, {800, "left", 0, false}, {1330, "right", 0, false}},
		},
		{
			name: "late start, early switch, neutral gap, late switch",
			player: []Segment{
				{0, 60, ""}, {60, 770, "right"}, {770, 1330, "left"}, {1330, 1375, ""}, {1375, 2210, "right"},
			},
			end:     2210,
			want:    []Change{{0, "right", 60, false}, {800, "left", -30, false}, {1330, "right", 45, false}},
			average: 45,
		},
		{
			name:    "never switches",
			player:  []Segment{{0, 2210, "right"}},
			end:     2210,
			want:    []Change{{0, "right", 0, false}, {800, "left", 0, true}, {1330, "right", 0, true}},
			average: 0,
			missed:  2,
		},
		{
			name:   "released before the second switch could be judged",
			player: []Segment{{0, 900, "right"}},
			end:    900,
			want:   []Change{{0, "right", 0, false}},
		},
		{
			name:    "released after the switch window closed",
			player:  []Segment{{0, 1200, "right"}},
			end:     1200,
			want:    []Change{{0, "right", 0, false}, {800, "left", 0, true}},
			average: 0,
			missed:  1,
		},
		{
			name:   "wrong way throughout",
			player: []Segment{{0, 2210, "left"}},
			end:    2210,
			want:   []Change{{0, "right", 0, true}, {800, "left", 0, true}, {1330, "right", 0, true}},
			missed: 3,
		},
		{
			name:    "switched far too early",
			player:  []Segment{{0, 300, "right"}, {300, 2210, "left"}},
			end:     2210,
			want:    []Change{{0, "right", 0, false}, {800, "left", -400, false}, {1330, "right", 0, true}},
			average: 200,
			missed:  1,
		},
		{
			name:    "picks the nearest of several entries",
			player:  []Segment{{0, 500, "right"}, {500, 520, "left"}, {520, 790, "right"}, {790, 2210, "left"}},
			end:     2210,
			want:    []Change{{0, "right", 0, false}, {800, "left", -10, false}, {1330, "right", 0, true}},
			average: 5,
			missed:  1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Evaluate(R301(), test.player, test.end)
			if len(got.Switches) != len(test.want) {
				t.Fatalf("switches = %+v, want %+v", got.Switches, test.want)
			}
			for i, want := range test.want {
				if got.Switches[i] != want {
					t.Errorf("switch %d = %+v, want %+v", i, got.Switches[i], want)
				}
			}
			if got.AverageMS != test.average || got.Missed != test.missed {
				t.Errorf("average %d missed %d, want %d and %d", got.AverageMS, got.Missed, test.average, test.missed)
			}
		})
	}
}

func TestEvaluateSkipsNeutralPhases(t *testing.T) {
	phases := []Phase{{"", 350}, {"right", 500}, {"left", 500}}
	got := Evaluate(phases, []Segment{{0, 370, ""}, {370, 850, "right"}, {850, 1350, "left"}}, 1350)
	want := []Change{{350, "right", 20, false}, {850, "left", 0, false}}
	if len(got.Switches) != 2 || got.Switches[0] != want[0] || got.Switches[1] != want[1] || got.AverageMS != 10 {
		t.Fatalf("got %+v", got)
	}
}
