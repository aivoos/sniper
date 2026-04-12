package aggregate

import "rlangga/internal/store"

// Stats aggregates closed-trade metrics (PR-003).
type Stats struct {
	Total    int
	Win      int
	Loss     int
	TotalPnL float64
	AvgPnL   float64
	Winrate  float64
}

// ComputeStats computes win rate, totals, and averages from trades (newest-first order does not matter for sums).
func ComputeStats(trades []store.Trade) Stats {
	s := Stats{}
	for _, t := range trades {
		s.Total++
		s.TotalPnL += t.PnLSOL
		if t.PnLSOL > 0 {
			s.Win++
		} else {
			s.Loss++
		}
	}
	if s.Total > 0 {
		s.AvgPnL = s.TotalPnL / float64(s.Total)
		s.Winrate = float64(s.Win) / float64(s.Total) * 100
	}
	return s
}

// LossStreak counts consecutive losing trades from the front of the slice (newest first).
func LossStreak(trades []store.Trade) int {
	streak := 0
	for _, t := range trades {
		if t.PnLSOL < 0 {
			streak++
		} else {
			break
		}
	}
	return streak
}
