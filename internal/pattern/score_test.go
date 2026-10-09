package pattern

import "testing"

func TestEvaluate(t *testing.T) {
	tests := []struct {
		name   string
		player []Segment
		end    int64
		want   []Change
		total  int
		missed int
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
			end:   2210,
			want:  []Change{{0, "right", 60, false}, {800, "left", -30, false}, {1330, "right", 45, false}},
			total: 135,
		},
		{
			name:   "never switches",
			player: []Segment{{0, 2210, "right"}},
			end:    2210,
			want:   []Change{{0, "right", 0, false}, {800, "left", 0, true}, {1330, "right", 0, true}},
			total:  0,
			missed: 2,
		},
		{
			name:   "released before the second switch could be judged",
			player: []Segment{{0, 900, "right"}},
			end:    900,
			want:   []Change{{0, "right", 0, false}},
		},
		{
			name:   "released after the switch window closed",
			player: []Segment{{0, 1200, "right"}},
			end:    1200,
			want:   []Change{{0, "right", 0, false}, {800, "left", 0, true}},
			total:  0,
			missed: 1,
		},
		{
			name:   "wrong way throughout",
			player: []Segment{{0, 2210, "left"}},
			end:    2210,
			want:   []Change{{0, "right", 0, true}, {800, "left", 0, true}, {1330, "right", 0, true}},
			missed: 3,
		},
		{
			name:   "switched far too early",
			player: []Segment{{0, 300, "right"}, {300, 2210, "left"}},
			end:    2210,
			want:   []Change{{0, "right", 0, false}, {800, "left", -400, false}, {1330, "right", 0, true}},
			total:  400,
			missed: 1,
		},
		{
			name:   "picks the nearest of several entries",
			player: []Segment{{0, 500, "right"}, {500, 520, "left"}, {520, 790, "right"}, {790, 2210, "left"}},
			end:    2210,
			want:   []Change{{0, "right", 0, false}, {800, "left", -10, false}, {1330, "right", 0, true}},
			total:  10,
			missed: 1,
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
			if got.TotalMS != test.total || got.Missed != test.missed {
				t.Errorf("total %d missed %d, want %d and %d", got.TotalMS, got.Missed, test.total, test.missed)
			}
		})
	}
}

func TestNeutralPhaseAcceptsAnyInput(t *testing.T) {
	phases := []Phase{{"", 350}, {"right", 370}, {"left", 350}}
	seg := func(from, to int64, direction string) Segment { return Segment{from, to, direction} }
	tests := []struct {
		name   string
		player []Segment
		want   Change
	}{
		{"held right throughout the wait", []Segment{seg(0, 720, "right"), seg(720, 1070, "left")}, Change{350, "right", 0, false}},
		{"started right during the wait", []Segment{seg(0, 200, ""), seg(200, 720, "right"), seg(720, 1070, "left")}, Change{350, "right", 0, false}},
		{"held left during the wait, switched late", []Segment{seg(0, 390, "left"), seg(390, 720, "right"), seg(720, 1070, "left")}, Change{350, "right", 40, false}},
		{"tapped right during the wait only", []Segment{seg(0, 100, "right"), seg(100, 720, ""), seg(720, 1070, "left")}, Change{350, "right", 0, true}},
	}
	for _, test := range tests {
		got := Evaluate(phases, test.player, 1070)
		if len(got.Switches) != 2 || got.Switches[0] != test.want {
			t.Errorf("%s: %+v", test.name, got.Switches)
		}
	}
}

func TestEvaluateSkipsNeutralPhases(t *testing.T) {
	phases := []Phase{{"", 350}, {"right", 500}, {"left", 500}}
	got := Evaluate(phases, []Segment{{0, 370, ""}, {370, 850, "right"}, {850, 1350, "left"}}, 1350)
	want := []Change{{350, "right", 20, false}, {850, "left", 0, false}}
	if len(got.Switches) != 2 || got.Switches[0] != want[0] || got.Switches[1] != want[1] || got.TotalMS != 20 {
		t.Fatalf("got %+v", got)
	}
}

func TestWeaponCatalogue(t *testing.T) {
	ids := map[string]bool{}
	for _, weapon := range Weapons() {
		if weapon.ID == "" || weapon.Name == "" || len(weapon.Modes) == 0 || ids[weapon.ID] {
			t.Fatalf("bad weapon %+v", weapon)
		}
		ids[weapon.ID] = true
		for _, mode := range weapon.Modes {
			if mode.ID == "" || mode.Name == "" || len(mode.Phases) == 0 {
				t.Fatalf("%s: bad mode %+v", weapon.ID, mode)
			}
			for i, phase := range mode.Phases {
				if phase.DurationMS <= 0 || (i > 0 && phase.Direction == mode.Phases[i-1].Direction) {
					t.Fatalf("%s/%s: phase %d does not change the strafe: %+v", weapon.ID, mode.ID, i, mode.Phases)
				}
			}
		}
	}
	if len(ids) != 15 {
		t.Fatalf("got %d weapons", len(ids))
	}
	if weapon, mode := Find("havoc", "turbocharged"); weapon.Name != "HAVOC" || TotalMS(mode.Phases) != 2500 {
		t.Fatalf("find: %+v %+v", weapon, mode)
	}
	if weapon, mode := Find("devotion", "nope"); weapon.ID != "devotion" || mode.ID != "normal" {
		t.Fatalf("unknown mode should fall back to the first: %+v", mode)
	}
	if weapon, _ := Find("", ""); weapon.ID == "" {
		t.Fatal("unknown weapon should fall back to the first")
	}
}
