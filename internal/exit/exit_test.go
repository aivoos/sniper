package exit

import (
	"testing"

	"rlangga/internal/bot"
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

func TestShouldSellAdaptive_NilCfg(t *testing.T) {
	if ShouldSellAdaptive(1, 1, &PositionState{}, nil) {
		t.Fatal("nil cfg")
	}
}

func TestShouldSellAdaptive_NilState(t *testing.T) {
	if ShouldSellAdaptive(1, 1, nil, cfgExit()) {
		t.Fatal("nil state")
	}
}

func TestShouldSellAdaptiveBot_NilState(t *testing.T) {
	b := bot.FromConfig(cfgExit())
	if ShouldSellAdaptiveBot(1, 1, nil, b) {
		t.Fatal("nil state")
	}
}

func TestShouldSellAdaptiveBot_MatchesFromConfig(t *testing.T) {
	c := cfgExit()
	b := bot.FromConfig(c)
	for _, tc := range []struct {
		pnl     float64
		elapsed int
	}{
		{-9, 100},
		{3, 1},
	} {
		s1 := &PositionState{}
		s2 := &PositionState{}
		if g, h := ShouldSellAdaptive(tc.pnl, tc.elapsed, s1, c), ShouldSellAdaptiveBot(tc.pnl, tc.elapsed, s2, b); g != h {
			t.Fatalf("mismatch pnl=%v elapsed=%v: %v vs %v", tc.pnl, tc.elapsed, g, h)
		}
	}
}

func TestShouldSellAdaptive_StopLossAfterGrace(t *testing.T) {
	st := &PositionState{}
	if !ShouldSellAdaptive(-6, 5, st, cfgExit()) {
		t.Fatal("stop loss after grace")
	}
}

func TestShouldSellAdaptive_TPAfterMinHold(t *testing.T) {
	st := &PositionState{}
	if !ShouldSellAdaptive(8, 6, st, cfgExit()) {
		t.Fatal("take profit when elapsed >= min hold")
	}
}

func TestShouldSellAdaptive_PeakTracking(t *testing.T) {
	st := &PositionState{}
	_ = ShouldSellAdaptive(5, 10, st, cfgExit())
	if st.PeakPnL != 5 {
		t.Fatalf("peak: %v", st.PeakPnL)
	}
}

func TestAdaptiveExitReason_GraceSL(t *testing.T) {
	c := cfgExit()
	c.GraceSL = 3
	b := bot.FromConfig(c)
	st := &PositionState{}
	if ok, r := AdaptiveExitReason(-4, 1, st, b); !ok || r != ExitGraceSL {
		t.Fatalf("expected grace_sl, got ok=%v r=%q", ok, r)
	}
	// PnL = -2 should NOT trigger grace_sl (below threshold)
	st2 := &PositionState{}
	if ok, _ := AdaptiveExitReason(-2, 1, st2, b); ok {
		t.Fatal("should not exit at -2 with grace_sl=3")
	}
}

func TestAdaptiveExitReason_GraceSL_DisabledByDefault(t *testing.T) {
	b := bot.FromConfig(cfgExit()) // GraceSL = 0, disabled
	st := &PositionState{}
	if ok, _ := AdaptiveExitReason(-4, 1, st, b); ok {
		t.Fatal("grace_sl=0 should not trigger exit")
	}
}

func TestNeedsConfirmation(t *testing.T) {
	if !NeedsConfirmation(ExitStopLoss) || !NeedsConfirmation(ExitGraceSL) || !NeedsConfirmation(ExitMomentum) {
		t.Fatal("loss-type reasons need confirmation")
	}
	if NeedsConfirmation(ExitTakeProfit) || NeedsConfirmation(ExitPanic) || NeedsConfirmation("other") {
		t.Fatal("profit/immediate reasons skip confirmation")
	}
}

func TestAdaptiveExitReason_Reasons(t *testing.T) {
	b := bot.FromConfig(cfgExit())
	st := &PositionState{}
	if ok, r := AdaptiveExitReason(-9, 5, st, b); !ok || r != ExitPanic {
		t.Fatalf("panic: ok=%v r=%q", ok, r)
	}
	st = &PositionState{}
	if ok, r := AdaptiveExitReason(8, 1, st, b); !ok || r != ExitGraceTP {
		t.Fatalf("grace_tp: ok=%v r=%q", ok, r)
	}
	st = &PositionState{}
	if ok, r := AdaptiveExitReason(-6, 5, st, b); !ok || r != ExitStopLoss {
		t.Fatalf("stop_loss: ok=%v r=%q", ok, r)
	}
	st = &PositionState{}
	if ok, r := AdaptiveExitReason(8, 6, st, b); !ok || r != ExitTakeProfit {
		t.Fatalf("take_profit: ok=%v r=%q", ok, r)
	}
	st = &PositionState{PeakPnL: 10}
	// Di bawah TP (7) supaya bukan take_profit; turun dari peak ≥ MomentumDrop.
	if ok, r := AdaptiveExitReason(6, 6, st, b); !ok || r != ExitMomentum {
		t.Fatalf("momentum: ok=%v r=%q", ok, r)
	}
	st = &PositionState{}
	c := cfgExit()
	c.GraceSeconds = 0
	c.MaxHold = 0
	b2 := bot.FromConfig(c)
	if ok, r := AdaptiveExitReason(0, 0, st, b2); !ok || r != ExitMaxHold {
		t.Fatalf("max_hold: ok=%v r=%q", ok, r)
	}
}
