package exit

import "rlangga/internal/config"

// PositionState tracks peak PnL for momentum exit.
type PositionState struct {
	BuySOL  float64
	PeakPnL float64
}

// ShouldSellAdaptive implements PR-002 exit engine (exit = pnl + momentum + time).
func ShouldSellAdaptive(pnl float64, elapsed int, state *PositionState, cfg *config.Config) bool {
	if cfg == nil || state == nil {
		return false
	}

	if pnl > state.PeakPnL {
		state.PeakPnL = pnl
	}

	if pnl <= -cfg.PanicSL {
		return true
	}

	if elapsed < cfg.GraceSeconds {
		return pnl >= cfg.TakeProfit
	}

	if pnl <= -cfg.StopLoss {
		return true
	}

	if pnl >= cfg.TakeProfit && elapsed >= cfg.MinHold {
		return true
	}

	drop := state.PeakPnL - pnl
	if drop >= cfg.MomentumDrop {
		return true
	}

	if elapsed >= cfg.MaxHold {
		return true
	}

	return false
}
