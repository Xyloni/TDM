package runner

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestFarmingHistoryMeasuresProgressAcrossRewardsWithoutCountingBaseline(t *testing.T) {
	var h farmHistory
	base := time.Unix(100000, 0)
	for m := 0; m <= 16; m++ {
		h.observe(base.Add(time.Duration(m)*time.Minute), "one", 100+m)
	}
	s := h.stats(base.Add(16 * time.Minute))
	require.Equal(t, 15, s.Minutes15.Gain)
	require.True(t, s.Minutes15.Ready)
	require.InDelta(t, 1, *s.Minutes15.Rate, .001)
	h.breakWatch()
	h.observe(base.Add(17*time.Minute), "two", 200)
	h.observe(base.Add(18*time.Minute), "two", 201)
	s = h.stats(base.Add(18 * time.Minute))
	require.Equal(t, 14, s.Minutes15.Gain, "a reward's initial 200 minutes must not be added")
	require.Equal(t, 14.0, s.Minutes15.ObservationMinutes)
}

func TestFarmingHistoryExcludesStoppedTimeAndInvalidSamples(t *testing.T) {
	var h farmHistory
	base := time.Unix(100000, 0)
	h.observe(base, "one", 10)
	h.observe(base.Add(time.Minute), "one", 11)
	h.observe(base.Add(5*time.Minute), "one", 15)  // observation gap
	h.observe(base.Add(6*time.Minute), "one", 1)   // regression
	h.observe(base.Add(7*time.Minute), "two", 100) // new reward
	h.observe(base.Add(8*time.Minute), "two", 101)
	s := h.stats(base.Add(8 * time.Minute))
	require.Equal(t, 2, s.Minutes15.Gain)
	require.Equal(t, 2.0, s.Minutes15.ObservationMinutes)
	require.Nil(t, s.Minutes15.Rate)
	h.breakWatch()
	h.observe(base.Add(9*time.Minute), "two", 102)
	require.Equal(t, 2, h.stats(base.Add(9*time.Minute)).Minutes15.Gain)
}

func TestFarmingHistoryBoundsMemoryAndRejectsStaleRates(t *testing.T) {
	var h farmHistory
	base := time.Unix(100000, 0)
	for m := 0; m < 10000; m++ {
		h.observe(base.Add(time.Duration(m)*time.Minute), "one", m)
	}
	require.LessOrEqual(t, len(h.intervals), 33)
	last := base.Add(9999 * time.Minute)
	s := h.stats(last)
	require.Equal(t, 30, s.Minutes30.Gain)
	require.True(t, s.Minutes30.Ready)
	s = h.stats(last.Add(3 * time.Minute))
	require.Nil(t, s.Minutes15.Rate)
	require.False(t, s.Minutes15.Ready)
	h.rotations = []time.Time{last.Add(-16 * time.Minute), last.Add(-time.Minute)}
	require.Equal(t, 1, h.stats(last).Rotations15)
}
