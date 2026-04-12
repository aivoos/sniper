package aggregate

import (
	"testing"

	"rlangga/internal/store"
)

func TestComputeStats_Empty(t *testing.T) {
	s := ComputeStats(nil)
	if s.Total != 0 || s.Winrate != 0 {
		t.Fatalf("%+v", s)
	}
}

func TestComputeStats_Mixed(t *testing.T) {
	tr := []store.Trade{
		{PnLSOL: 0.01},
		{PnLSOL: -0.02},
		{PnLSOL: 0},
	}
	s := ComputeStats(tr)
	if s.Total != 3 || s.Win != 1 || s.Loss != 2 {
		t.Fatalf("win/loss: %+v", s)
	}
	if s.TotalPnL != -0.01 {
		t.Fatalf("total pnl: %v", s.TotalPnL)
	}
	expAvg := -0.01 / 3
	if s.AvgPnL < expAvg-1e-9 || s.AvgPnL > expAvg+1e-9 {
		t.Fatalf("avg: %v want ~%v", s.AvgPnL, expAvg)
	}
	expWR := 100.0 / 3
	if s.Winrate < expWR-0.01 || s.Winrate > expWR+0.01 {
		t.Fatalf("winrate: %v", s.Winrate)
	}
}

func TestLossStreak(t *testing.T) {
	tr := []store.Trade{
		{PnLSOL: -1},
		{PnLSOL: -2},
		{PnLSOL: 0.1},
		{PnLSOL: -3},
	}
	if LossStreak(tr) != 2 {
		t.Fatal("newest-first two losses")
	}
	if LossStreak([]store.Trade{{PnLSOL: 1}, {PnLSOL: -1}}) != 0 {
		t.Fatal("first win breaks")
	}
}
