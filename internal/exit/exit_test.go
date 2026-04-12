package exit

import (
	"testing"

	"rlangga/internal/config"
)

func cfgExit() *config.Config {
	return &config.Config{
		GraceSeconds:    2,
		MinHold:         5,
		MaxHold:         15,
		TakeProfit:      7,
		StopLoss:        5,
		PanicSL:         8,
		MomentumDrop:    2.5,
		QuoteIntervalMS: 500,
	}
}

func TestShouldSellAdaptive_Panic(t *testing.T) {
	st := &PositionState{}
	if !ShouldSellAdaptive(-9, 100, st, cfgExit()) {
		t.Fatal("panic SL")
	}
}

func TestShouldSellAdaptive_MomentumDrop(t *testing.T) {
	st := &PositionState{PeakPnL: 10}
	if !ShouldSellAdaptive(7, 6, st, cfgExit()) {
		t.Fatal("expected sell on momentum drop from peak 10")
	}
}

func TestShouldSellAdaptive_GraceNoise(t *testing.T) {
	st := &PositionState{}
	if ShouldSellAdaptive(3, 1, st, cfgExit()) {
		t.Fatal("in grace, below TP — no sell")
	}
}

func TestShouldSellAdaptive_MaxHold(t *testing.T) {
	st := &PositionState{}
	c := cfgExit()
	c.GraceSeconds = 0 // else grace window returns before max-hold check
	c.MaxHold = 0
	if !ShouldSellAdaptive(0, 0, st, c) {
		t.Fatal("max hold 0 should exit when grace disabled")
	}
}
