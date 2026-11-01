package runner

import "time"

// FarmingWindow measures platform progress, excluding gaps in observations.
type FarmingWindow struct {
	Gain               int      `json:"gain"`
	ObservationMinutes float64  `json:"observation_minutes"`
	Rate               *float64 `json:"rate"`
	Ready              bool     `json:"ready"`
}

type FarmingStats struct {
	Minutes15   FarmingWindow `json:"minutes_15"`
	Minutes30   FarmingWindow `json:"minutes_30"`
	Rotations15 int           `json:"rotations_15"`
}

type farmInterval struct {
	start, end time.Time
	gain       int
}
type farmHistory struct {
	last      time.Time
	benefit   string
	minutes   int
	intervals []farmInterval
	rotations []time.Time
}

func (h *farmHistory) breakWatch() { h.last = time.Time{} }

func (h *farmHistory) observe(now time.Time, benefit string, minutes int) {
	if !h.last.IsZero() && benefit == h.benefit && now.After(h.last) && now.Sub(h.last) <= slowAccrualMaxGap && minutes >= h.minutes {
		h.intervals = append(h.intervals, farmInterval{h.last, now, minutes - h.minutes})
	}
	h.last, h.benefit, h.minutes = now, benefit, minutes
	h.prune(now)
}

func (h *farmHistory) prune(now time.Time) {
	cut := now.Add(-31 * time.Minute)
	i := 0
	for i < len(h.intervals) && h.intervals[i].end.Before(cut) {
		i++
	}
	if i > 0 {
		copy(h.intervals, h.intervals[i:])
		h.intervals = h.intervals[:len(h.intervals)-i]
	}
	i = 0
	for i < len(h.rotations) && h.rotations[i].Before(now.Add(-15*time.Minute)) {
		i++
	}
	if i > 0 {
		copy(h.rotations, h.rotations[i:])
		h.rotations = h.rotations[:len(h.rotations)-i]
	}
}

func (h *farmHistory) stats(now time.Time) FarmingStats {
	h.prune(now)
	window := func(d time.Duration) FarmingWindow {
		w := FarmingWindow{}
		cut := now.Add(-d)
		for _, x := range h.intervals {
			// Whole intervals only: never invent fractional credited minutes.
			if x.start.Before(cut) || x.end.After(now) {
				continue
			}
			w.Gain += x.gain
			w.ObservationMinutes += x.end.Sub(x.start).Minutes()
		}
		fresh := !h.last.IsZero() && now.Sub(h.last) <= slowAccrualMaxGap
		if w.ObservationMinutes >= 5 && fresh {
			rate := float64(w.Gain) / w.ObservationMinutes
			w.Rate = &rate
		}
		w.Ready = fresh && w.ObservationMinutes >= d.Minutes()-1.5
		return w
	}
	return FarmingStats{Minutes15: window(15 * time.Minute), Minutes30: window(30 * time.Minute), Rotations15: len(h.rotations)}
}
