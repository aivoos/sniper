package exit

import (
	"rlangga/internal/bot"
	"rlangga/internal/config"
)

// PositionState tracks peak PnL for momentum exit.
type PositionState struct {
	BuySOL  float64
	PeakPnL float64
}

// ShouldSellAdaptive implements PR-002 exit engine using global config as one profile.
func ShouldSellAdaptive(pnl float64, elapsed int, state *PositionState, cfg *config.Config) bool {
	if cfg == nil || state == nil {
		return false
	}
	return ShouldSellAdaptiveBot(pnl, elapsed, state, bot.FromConfig(cfg))
}

// ShouldSellAdaptiveBot is PR-002 exit rules with thresholds from a bot profile (PR-004).
func ShouldSellAdaptiveBot(pnl float64, elapsed int, state *PositionState, b bot.BotConfig) bool {
	if state == nil {
		return false
	}

	if pnl > state.PeakPnL {
		state.PeakPnL = pnl
	}

	if pnl <= -b.PanicLoss {
		return true
	}

	if elapsed < b.GraceSeconds {
		return pnl >= b.TakeProfit
	}

	if pnl <= -b.StopLoss {
		return true
	}

	if pnl >= b.TakeProfit && elapsed >= b.MinHold {
		return true
	}

	drop := state.PeakPnL - pnl
	if drop >= b.MomentumDrop {
		return true
	}

	if elapsed >= b.MaxHold {
		return true
	}

	return false
}
