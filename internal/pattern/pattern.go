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

// Cues returns one cue per phase, leadMS before the phase starts and never
// before time zero.
func Cues(phases []Phase, leadMS int) []Cue {
	cues := make([]Cue, 0, len(phases))
	var start int64
	for _, phase := range phases {
		cues = append(cues, Cue{AtMS: max(start-int64(leadMS), 0), Direction: phase.Direction})
		start += int64(phase.DurationMS)
	}
	return cues
}
