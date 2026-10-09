package pattern

// Segment is a stretch of time the player spent strafing one way. An empty
// Direction is neutral: no strafe key held, or both at once.
type Segment struct {
	StartMS   int64  `json:"startMs"`
	EndMS     int64  `json:"endMs"`
	Direction string `json:"direction"`
}

// Change compares one expected direction change with what the player did.
// DeviationMS is negative when the player moved early and positive when late.
type Change struct {
	AtMS        int64  `json:"atMs"`
	Direction   string `json:"direction"`
	DeviationMS int64  `json:"deviationMs"`
	Missed      bool   `json:"missed"`
}

// Score summarises a spray. TotalMS is the summed absolute deviation of the
// switches the player made; missed switches are counted separately.
type Score struct {
	Switches []Change `json:"switches"`
	TotalMS  int      `json:"totalMs"`
	Missed   int      `json:"missed"`
}

// Evaluate scores the player's strafes against phases for a spray that ended
// at endMS. Each phase start is a switch, matched to the nearest moment the
// player began strafing that way within half a phase either side. Switches the
// spray ended too early to judge are left out, as are neutral phases.
//
// A neutral phase accepts any input, so the switch that follows it cannot be
// early: already strafing the right way when it arrives counts as on time.
func Evaluate(phases []Phase, player []Segment, endMS int64) Score {
	score := Score{Switches: []Change{}}
	var at, total int64
	for i, phase := range phases {
		at += func() int64 {
			if i == 0 {
				return 0
			}
			return int64(phases[i-1].DurationMS)
		}()
		if at > endMS {
			break
		}
		if phase.Direction != "left" && phase.Direction != "right" {
			continue
		}
		from, previous := at, at
		if i > 0 {
			from -= int64(phases[i-1].DurationMS) / 2
			previous -= int64(phases[i-1].DurationMS)
		}
		to := at + int64(phase.DurationMS)/2

		afterNeutral := i > 0 && phases[i-1].Direction == ""
		found, best := false, int64(0)
		for _, segment := range player {
			if segment.Direction != phase.Direction {
				continue
			}
			deviation := segment.StartMS - at
			switch {
			case afterNeutral && segment.StartMS <= at && segment.EndMS > at:
				deviation = 0
			case afterNeutral && segment.StartMS < at:
				continue // a strafe that ended during the neutral phase says nothing
			case segment.StartMS >= from && segment.StartMS <= to:
			case segment.StartMS > previous && segment.StartMS < from && segment.EndMS > at:
				// Switched during the previous phase, before the window opened.
				deviation = from - at
			default:
				continue
			}
			if !found || abs(deviation) < abs(best) {
				found, best = true, deviation
			}
		}
		if !found && endMS < to {
			continue // released before a late switch could be ruled out
		}
		score.Switches = append(score.Switches, Change{AtMS: at, Direction: phase.Direction, DeviationMS: best, Missed: !found})
		if found {
			total += abs(best)
		} else {
			score.Missed++
		}
	}
	if made := len(score.Switches) - score.Missed; made > 0 {
		score.TotalMS = int(total)
	}
	return score
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
