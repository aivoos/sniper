package exit

import (
	"rlangga/internal/bot"
	"rlangga/internal/config"
)

// Alasan exit adaptif (PR-002) — dipakai log / store.
const (
	ExitPanic      = "panic"
	ExitGraceTP    = "grace_tp"
	ExitGraceSL    = "grace_sl"
	ExitStopLoss   = "stop_loss"
	ExitTakeProfit = "take_profit"
	ExitMomentum   = "momentum"
	ExitMaxHold    = "max_hold"
	ExitRugRemove  = "rug_remove_liquidity"
	ExitWhaleDump  = "whale_dump"
)

// NeedsConfirmation returns true for loss-type exits that benefit from anti-wick delay.
// Profit exits (grace_tp, take_profit) and max_hold execute immediately.
func NeedsConfirmation(reason string) bool {
	switch reason {
	case ExitStopLoss, ExitGraceSL, ExitMomentum:
		return true
	}
	return false
}

// PositionState tracks peak PnL for momentum exit and grace trailing.
type PositionState struct {
	BuySOL        float64
	PeakPnL       float64
	GraceTrailing bool
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
	ok, _ := AdaptiveExitReason(pnl, elapsed, state, b)
	return ok
}

// AdaptiveExitReason mengembalikan apakah harus jual dan label alasan (urutan cek sama ShouldSellAdaptiveBot).
func AdaptiveExitReason(pnl float64, elapsed int, state *PositionState, b bot.BotConfig) (sell bool, reason string) {
	if state == nil {
		return false, ""
	}

	if pnl > state.PeakPnL {
		state.PeakPnL = pnl
	}

	if pnl <= -b.PanicLoss {
		return true, ExitPanic
	}

	if elapsed < b.GraceSeconds || state.GraceTrailing {
		graceTP := b.GraceTP
		if graceTP <= 0 {
			graceTP = b.TakeProfit
		}

		if b.GraceTrailDrop > 0 && (pnl >= graceTP || state.GraceTrailing) {
			state.GraceTrailing = true
			drop := state.PeakPnL - pnl
			if drop >= b.GraceTrailDrop {
				return true, ExitGraceTP
			}
			return false, ""
		}

		if pnl >= graceTP {
			return true, ExitGraceTP
		}
		if elapsed < b.GraceSeconds {
			if b.GraceSL > 0 && pnl <= -b.GraceSL {
				return true, ExitGraceSL
			}
			return false, ""
		}
	}

	if pnl <= -b.StopLoss {
		return true, ExitStopLoss
	}

	if pnl >= b.TakeProfit && elapsed >= b.MinHold {
		return true, ExitTakeProfit
	}

	drop := state.PeakPnL - pnl
	if drop >= b.MomentumDrop {
		return true, ExitMomentum
	}

	if elapsed >= b.MaxHold {
		return true, ExitMaxHold
	}

	return false, ""
}
