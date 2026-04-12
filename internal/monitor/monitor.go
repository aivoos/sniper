package monitor

import (
	"time"

	"rlangga/internal/config"
	"rlangga/internal/executor"
	"rlangga/internal/exit"
	"rlangga/internal/pnl"
	"rlangga/internal/quote"
	"rlangga/internal/report"
	"rlangga/internal/store"
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
			if !executor.SafeSellWithValidation(mint) {
				return
			}
			sellSOL := quote.GetSellQuote(mint)
			ts := time.Now().Unix()
			pnlSOL := sellSOL - buySOL
			pctSOL := 0.0
			if buySOL > 0 {
				pctSOL = (pnlSOL / buySOL) * 100
			}
			dur := int(time.Since(start).Seconds())
			_ = store.SaveTrade(store.Trade{
				Mint:        mint,
				BuySOL:      buySOL,
				SellSOL:     sellSOL,
				PnLSOL:      pnlSOL,
				Percent:     pctSOL,
				DurationSec: dur,
				TS:          ts,
			})
			_ = report.NotifyTradeSaved()
			return
		}

		time.Sleep(interval)
	}
}
