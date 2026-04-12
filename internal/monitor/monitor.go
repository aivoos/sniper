package monitor

import (
	"fmt"
	"time"

	"rlangga/internal/bot"
	"rlangga/internal/config"
	"rlangga/internal/executor"
	"rlangga/internal/exit"
	"rlangga/internal/guard"
	"rlangga/internal/log"
	"rlangga/internal/pnl"
	"rlangga/internal/quote"
	"rlangga/internal/report"
	"rlangga/internal/store"
)

// MonitorPosition polls quote until adaptive exit triggers, then sells (global config profile).
func MonitorPosition(mint string, buySOL float64) {
	cfg := config.C
	if cfg == nil {
		return
	}
	MonitorPositionWithBot(mint, buySOL, bot.FromConfig(cfg))
}

// MonitorPositionWithBot uses exit thresholds from the given bot profile (PR-004); quote interval from config.C.
func MonitorPositionWithBot(mint string, buySOL float64, b bot.BotConfig) {
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

		if exit.ShouldSellAdaptiveBot(pct, elapsed, state, b) {
			log.Info(fmt.Sprintf("[%s] SELL %s", b.Name, mint))
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
			saved, err := store.SaveTrade(store.Trade{
				Mint:        mint,
				BotName:     b.Name,
				BuySOL:      buySOL,
				SellSOL:     sellSOL,
				PnLSOL:      pnlSOL,
				Percent:     pctSOL,
				DurationSec: dur,
				TS:          ts,
			})
			if err == nil && saved {
				_ = guard.UpdateDailyLoss(pnlSOL)
			}
			_ = report.NotifyTradeSaved()
			return
		}

		time.Sleep(interval)
	}
}
