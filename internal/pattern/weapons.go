package pattern

// Weapon is one gun and its firing modes, each with its own strafe pattern.
type Weapon struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Modes    []Mode `json:"modes"`
}

// Mode is one way of firing a weapon, such as a charged or turbocharged spray.
type Mode struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Phases []Phase `json:"phases"`
}

// The presets, transcribed from the public pattern data of
// https://www.apexrecoilstrafing.com/ on 2026-10-04. They are that trainer's
// presets, not measured game telemetry. Two adjustments were made: its "shoot"
// steps (fire without strafing) are neutral phases here, and consecutive steps
// in the same direction are merged, since they ask for no change of strafe.
var weapons = []Weapon{
	{ID: "flatline", Name: "Flatline", Category: "ar", Modes: []Mode{
		{ID: "default", Name: "Default", Phases: []Phase{{"left", 600}, {"right", 500}, {"left", 1100}, {"right", 610}}},
	}},
	{ID: "havoc", Name: "HAVOC", Category: "ar", Modes: []Mode{
		{ID: "normal", Name: "Normal", Phases: []Phase{{"", 350}, {"right", 370}, {"left", 350}, {"right", 630}, {"left", 1150}}},
		{ID: "turbocharged", Name: "Turbocharged", Phases: []Phase{{"right", 370}, {"left", 350}, {"right", 630}, {"left", 1150}}},
	}},
	{ID: "hemlok", Name: "Hemlok", Category: "ar", Modes: []Mode{
		{ID: "default", Name: "Default", Phases: []Phase{{"left", 1310}, {"right", 660}, {"left", 650}, {"right", 660}, {"left", 390}}},
	}},
	{ID: "nemesis", Name: "Nemesis", Category: "ar", Modes: []Mode{
		{ID: "charged", Name: "Charged", Phases: []Phase{{"right", 330}, {"left", 660}, {"right", 330}, {"left", 660}, {"right", 330}, {"left", 330}}},
	}},
	{ID: "r301", Name: "R-301", Category: "ar", Modes: []Mode{
		{ID: "default", Name: "Default", Phases: []Phase{{"right", 800}, {"left", 530}, {"right", 880}}},
	}},
	{ID: "alternator", Name: "Alternator", Category: "smg", Modes: []Mode{
		{ID: "default", Name: "Default", Phases: []Phase{{"left", 600}, {"right", 700}, {"left", 800}, {"right", 710}}},
		{ID: "doubletap", Name: "Double Tap", Phases: []Phase{{"left", 450}, {"right", 525}, {"left", 600}, {"right", 530}}},
	}},
	{ID: "car", Name: "C.A.R.", Category: "smg", Modes: []Mode{
		{ID: "default", Name: "Default", Phases: []Phase{{"left", 470}, {"right", 380}, {"left", 250}, {"right", 330}, {"left", 320}}},
	}},
	{ID: "prowler", Name: "Prowler", Category: "smg", Modes: []Mode{
		{ID: "burst", Name: "Burst", Phases: []Phase{{"left", 1280}, {"right", 1050}, {"left", 410}}},
		{ID: "auto", Name: "Auto", Phases: []Phase{{"left", 990}, {"right", 660}, {"left", 380}, {"right", 530}}},
	}},
	{ID: "r99", Name: "R-99", Category: "smg", Modes: []Mode{
		{ID: "default", Name: "Default", Phases: []Phase{{"right", 390}, {"left", 270}, {"right", 290}, {"left", 220}, {"right", 280}}},
	}},
	{ID: "volt", Name: "Volt", Category: "smg", Modes: []Mode{
		{ID: "default", Name: "Default", Phases: []Phase{{"right", 750}, {"left", 420}, {"right", 250}, {"left", 750}}},
	}},
	{ID: "devotion", Name: "Devotion", Category: "lmg", Modes: []Mode{
		{ID: "normal", Name: "Normal", Phases: []Phase{{"right", 500}, {"left", 1190}, {"right", 1450}, {"left", 530}}},
		{ID: "turbocharged", Name: "Turbocharged", Phases: []Phase{{"right", 270}, {"left", 1080}, {"right", 1400}, {"left", 760}}},
	}},
	{ID: "lstar", Name: "L-STAR", Category: "lmg", Modes: []Mode{
		{ID: "default", Name: "Default", Phases: []Phase{{"left", 300}, {"right", 2400}}},
	}},
	{ID: "rampage", Name: "Rampage", Category: "lmg", Modes: []Mode{
		{ID: "normal", Name: "Normal", Phases: []Phase{{"left", 620}, {"right", 1230}, {"left", 6100}}},
		{ID: "charged", Name: "Charged", Phases: []Phase{{"left", 450}, {"right", 940}, {"left", 4600}}},
	}},
	{ID: "spitfire", Name: "Spitfire", Category: "lmg", Modes: []Mode{
		{ID: "default", Name: "Default", Phases: []Phase{{"left", 430}, {"right", 670}, {"left", 1110}, {"right", 1440}, {"left", 1330}, {"right", 460}}},
	}},
	{ID: "re45", Name: "RE-45", Category: "pistol", Modes: []Mode{
		{ID: "default", Name: "Default", Phases: []Phase{{"", 400}, {"left", 500}, {"right", 500}, {"left", 500}, {"right", 500}}},
	}},
}

// Weapons returns every preset. The list is shared: do not modify it.
func Weapons() []Weapon { return weapons }

// Find returns the weapon and mode with the given IDs, falling back to the
// weapon's first mode and then to the first weapon, so a stale or empty
// setting always resolves to a valid pattern.
func Find(weaponID, modeID string) (Weapon, Mode) {
	weapon := weapons[0]
	for _, candidate := range weapons {
		if candidate.ID == weaponID {
			weapon = candidate
		}
	}
	for _, mode := range weapon.Modes {
		if mode.ID == modeID {
			return weapon, mode
		}
	}
	return weapon, weapon.Modes[0]
}
