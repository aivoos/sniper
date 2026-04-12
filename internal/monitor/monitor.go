package monitor

import (
	"time"

	"rlangga/internal/config"
	"rlangga/internal/executor"
	"rlangga/internal/exit"
	"rlangga/internal/pnl"
	"rlangga/internal/quote"
)

// MonitorPosition polls quote until adaptive exit triggers, then sells.
func MonitorPosition(mint string, buySOL float64) {
	cfg := config.C
	if cfg == nil {
		return
	}

	state := &exit.PositionState{
		BuySOL:  buySOL,
		PeakPnL: 0,
	}
	start := time.Now()
	interval := time.Duration(cfg.QuoteIntervalMS) * time.Millisecond
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}

	for {
		elapsed := int(time.Since(start).Seconds())

		q := quote.GetSellQuote(mint)
		pct := pnl.CalcPnL(buySOL, q)

		if exit.ShouldSellAdaptive(pct, elapsed, state, cfg) {
			executor.SafeSellWithValidation(mint)
			return
		}

		time.Sleep(interval)
	}
}
