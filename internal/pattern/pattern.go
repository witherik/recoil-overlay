package pattern

// Phase describes one directional segment of a recoil pattern.
type Phase struct {
	Direction  string `json:"direction"`
	DurationMS int    `json:"durationMs"`
}

// Cue marks when a phase's direction should be announced.
type Cue struct {
	AtMS      int64
	Direction string
}

// R301 returns the standard three-phase R-301 timing pattern.
func R301() []Phase {
	return []Phase{
		{Direction: "right", DurationMS: 800},
		{Direction: "left", DurationMS: 530},
		{Direction: "right", DurationMS: 880},
	}
}

// TotalMS returns the sum of the phase durations.
func TotalMS(phases []Phase) int {
	total := 0
	for _, phase := range phases {
		total += phase.DurationMS
	}
	return total
}

// At returns the active phase at elapsedMS. Negative times select the first
// phase, while the total duration and all later times are considered done.
func At(phases []Phase, elapsedMS int64) (index int, direction string, done bool) {
	if len(phases) == 0 {
		return -1, "", true
	}
	if elapsedMS < 0 {
		return 0, phases[0].Direction, false
	}

	var boundary int64
	for i, phase := range phases {
		boundary += int64(phase.DurationMS)
		if elapsedMS < boundary {
			return i, phase.Direction, false
		}
	}
	return -1, "", true
}

// Cues returns a cue at time zero for the first phase and one for every later
// phase at its boundary minus leadMS. Later cues are clamped to time zero.
func Cues(phases []Phase, leadMS int) []Cue {
	if len(phases) == 0 {
		return nil
	}
	cues := make([]Cue, 0, len(phases))
	cues = append(cues, Cue{AtMS: 0, Direction: phases[0].Direction})
	var boundary int64
	for i, phase := range phases {
		boundary += int64(phase.DurationMS)
		if i == 0 {
			continue
		}
		at := boundary - int64(phase.DurationMS) - int64(leadMS)
		if at < 0 {
			at = 0
		}
		cues = append(cues, Cue{AtMS: at, Direction: phase.Direction})
	}
	return cues
}
