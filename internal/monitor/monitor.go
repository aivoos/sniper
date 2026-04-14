package monitor

import (
	"fmt"
	"runtime/debug"
	"time"

	"rlangga/internal/bot"
	"rlangga/internal/config"
	"rlangga/internal/executor"
	"rlangga/internal/exit"
	"rlangga/internal/guard"
	"rlangga/internal/lock"
	"rlangga/internal/log"
	"rlangga/internal/pnl"
	"rlangga/internal/pumpws"
	"rlangga/internal/quote"
	"rlangga/internal/report"
	"rlangga/internal/sellguard"
	"rlangga/internal/store"
)

// MonitorPosition polls quote until adaptive exit triggers, then sells (global config profile).
func MonitorPosition(mint string, buySOL float64) {
	cfg := config.C
	if cfg == nil {
		return
	}
	MonitorPositionWithBot(mint, buySOL, bot.FromConfig(cfg), nil, nil)
}

// MonitorPositionWithBot uses exit thresholds from the given bot profile (PR-004); quote interval from config.C.
// entry: snapshot stream pra-BUY (opsional) untuk kolom entry_* pada trade terekam.
// snap: snapshot tracker aktivitas saat BUY (buy/sell ratio, mcap rising) — opsional.
func MonitorPositionWithBot(mint string, buySOL float64, b bot.BotConfig, entry *pumpws.StreamEvent, snap *pumpws.ActivitySnapshot) {
	defer func() {
		if r := recover(); r != nil {
			log.Error(fmt.Sprintf("PANIC RECOVERED [monitor:%s]: %v\n%s", mint, r, debug.Stack()))
		}
	}()
	cfg := config.C
	if cfg == nil {
		return
	}

	evCh, cancelEv := pumpws.SubscribeMint(mint)
	defer cancelEv()

	var lastTickQuote float64
	var hasLastTickQuote bool

	state := &exit.PositionState{
		BuySOL:  buySOL,
		PeakPnL: 0,
	}
	start := time.Now()
	interval := time.Duration(cfg.QuoteIntervalMS) * time.Millisecond
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	lastLockRefresh := start

	confirmDur := time.Duration(cfg.ConfirmSLMS) * time.Millisecond
	var slConfirmSince time.Time
	var slConfirmReason string

	for {
		elapsed := int(time.Since(start).Seconds())

		if time.Since(lastLockRefresh) > 2*time.Minute {
			lock.RefreshMint(mint)
			lastLockRefresh = time.Now()
		}

		// Rug signal: any liquidity remove event for this mint triggers immediate exit.
		select {
		case ev, ok := <-evCh:
			if ok && ev.TxType == "remove" {
				// Override: sell immediately regardless of pnl/elapsed thresholds.
				exitReason := exit.ExitRugRemove
				if !sellguard.TryAcquireSellExit(mint) {
					log.Info(fmt.Sprintf("[%s] SELL skipped (exit lock held) %s", b.Name, mint))
					return
				}
				defer sellguard.ReleaseSellExit(mint)
				log.Info(fmt.Sprintf("[%s] SELL %s exit=%s", b.Name, mint, exitReason))
				if !executor.SafeSellWithValidation(mint) {
					return
				}
				sellSOL := quote.GetSellQuote(mint)
				if sellSOL <= 0 && hasLastTickQuote {
					sellSOL = lastTickQuote
				}
				if cfg.SimulateEngine && sellSOL <= 0 {
					sellSOL = quote.SyntheticSellQuoteForEngine(mint, buySOL, elapsed)
				}
				ts := time.Now().Unix()
				buyCost, sellProceeds := pnl.ApplyFees(buySOL, sellSOL, cfg.PumpFeeBuyPct, cfg.PumpFeeSellPct)
				pnlSOL := sellProceeds - buyCost
				pctSOL := 0.0
				if buyCost > 0 {
					pctSOL = (pnlSOL / buyCost) * 100
				}
				dur := int(time.Since(start).Seconds())
				tr := store.Trade{
					Mint:        mint,
					BotName:     b.Name,
					BuySOL:      buySOL,
					SellSOL:     sellSOL,
					PnLSOL:      pnlSOL,
					Percent:     pctSOL,
					DurationSec: dur,
					ExitReason:  exitReason,
					TS:          ts,
					BuyTS:       start.Unix(),
				}
				store.ApplyStreamEntryToTrade(&tr, entry)
				store.ApplyActivitySnapshotToTrade(&tr, snap)
				saved, err := store.SaveTrade(tr)
				report.LogTradeRealtime(tr, saved, err)
				if err == nil && saved {
					_ = guard.UpdateDailyLoss(pnlSOL)
				}
				if err := report.NotifyTradeSavedWithTrade(tr); err != nil {
					log.Error("report: NotifyTradeSaved: " + err.Error())
				}
				return
			}
			// Whale dump signal: large sells often precede sharp drawdowns; exit immediately.
			if ok && cfg.WhaleSellMinSOL > 0 && ev.TxType == "sell" && ev.HasSolAmount && ev.SolAmount >= cfg.WhaleSellMinSOL {
				exitReason := exit.ExitWhaleDump
				if !sellguard.TryAcquireSellExit(mint) {
					log.Info(fmt.Sprintf("[%s] SELL skipped (exit lock held) %s", b.Name, mint))
					return
				}
				defer sellguard.ReleaseSellExit(mint)
				log.Info(fmt.Sprintf("[%s] SELL %s exit=%s", b.Name, mint, exitReason))
				if !executor.SafeSellWithValidation(mint) {
					return
				}
				// Best effort: use quote tick below when available; otherwise use current quote endpoint.
				sellSOL := quote.GetSellQuote(mint)
				if sellSOL <= 0 && hasLastTickQuote {
					sellSOL = lastTickQuote
				}
				if cfg.SimulateEngine && sellSOL <= 0 {
					sellSOL = quote.SyntheticSellQuoteForEngine(mint, buySOL, elapsed)
				}
				ts := time.Now().Unix()
				buyCost, sellProceeds := pnl.ApplyFees(buySOL, sellSOL, cfg.PumpFeeBuyPct, cfg.PumpFeeSellPct)
				pnlSOL := sellProceeds - buyCost
				pctSOL := 0.0
				if buyCost > 0 {
					pctSOL = (pnlSOL / buyCost) * 100
				}
				dur := int(time.Since(start).Seconds())
				tr := store.Trade{
					Mint:        mint,
					BotName:     b.Name,
					BuySOL:      buySOL,
					SellSOL:     sellSOL,
					PnLSOL:      pnlSOL,
					Percent:     pctSOL,
					DurationSec: dur,
					ExitReason:  exitReason,
					TS:          ts,
					BuyTS:       start.Unix(),
				}
				store.ApplyStreamEntryToTrade(&tr, entry)
				store.ApplyActivitySnapshotToTrade(&tr, snap)
				saved, err := store.SaveTrade(tr)
				report.LogTradeRealtime(tr, saved, err)
				if err == nil && saved {
					_ = guard.UpdateDailyLoss(pnlSOL)
				}
				if err := report.NotifyTradeSavedWithTrade(tr); err != nil {
					log.Error("report: NotifyTradeSaved: " + err.Error())
				}
				return
			}
		default:
		}

		q, receivedAt := quote.GetSellQuoteWithTime(mint)
		if cfg.SimulateEngine && (q <= 0 || receivedAt.IsZero()) {
			q = quote.SyntheticSellQuoteForEngine(mint, buySOL, elapsed)
			receivedAt = time.Now()
		}
		if quoteStale(receivedAt, cfg.QuoteMaxAgeMS) {
			time.Sleep(interval)
			continue
		}
		if q > 0 {
			lastTickQuote = q
			hasLastTickQuote = true
		}
		buyCostForPct, qNet := pnl.ApplyFees(buySOL, q, cfg.PumpFeeBuyPct, cfg.PumpFeeSellPct)
		pct := pnl.CalcPnL(buyCostForPct, qNet)

		sell, exitReason := exit.AdaptiveExitReason(pct, elapsed, state, b)
		if sell && confirmDur > 0 && exit.NeedsConfirmation(exitReason) {
			if slConfirmSince.IsZero() {
				slConfirmSince = time.Now()
				slConfirmReason = exitReason
				log.Info(fmt.Sprintf("[%s] SL confirm started (reason=%s pnl=%.2f%%) waiting %dms mint=%s", b.Name, exitReason, pct, cfg.ConfirmSLMS, mint))
				time.Sleep(interval)
				continue
			}
			if time.Since(slConfirmSince) < confirmDur {
				time.Sleep(interval)
				continue
			}
			exitReason = slConfirmReason
		} else if sell && !slConfirmSince.IsZero() && !exit.NeedsConfirmation(exitReason) {
			log.Info(fmt.Sprintf("[%s] SL confirm escalated to %s (pnl=%.2f%%), immediate sell mint=%s", b.Name, exitReason, pct, mint))
			slConfirmSince = time.Time{}
			slConfirmReason = ""
		} else if !sell && !slConfirmSince.IsZero() {
			log.Info(fmt.Sprintf("[%s] SL confirm cancelled (wick recovered, pnl=%.2f%%) mint=%s", b.Name, pct, mint))
			slConfirmSince = time.Time{}
			slConfirmReason = ""
		}
		if sell {
			if !sellguard.TryAcquireSellExit(mint) {
				log.Info(fmt.Sprintf("[%s] SELL skipped (exit lock held) %s", b.Name, mint))
				return
			}
			defer sellguard.ReleaseSellExit(mint)
			log.Info(fmt.Sprintf("[%s] SELL %s exit=%s", b.Name, mint, exitReason))
			if !executor.SafeSellWithValidation(mint) {
				return
			}
			sellSOL := quote.GetSellQuote(mint)
			if sellSOL <= 0 {
				sellSOL = q
			}
			if cfg.SimulateEngine && sellSOL <= 0 {
				sellSOL = quote.SyntheticSellQuoteForEngine(mint, buySOL, elapsed)
			}
			ts := time.Now().Unix()
			buyCost, sellProceeds := pnl.ApplyFees(buySOL, sellSOL, cfg.PumpFeeBuyPct, cfg.PumpFeeSellPct)
			pnlSOL := sellProceeds - buyCost
			pctSOL := 0.0
			if buyCost > 0 {
				pctSOL = (pnlSOL / buyCost) * 100
			}
			dur := int(time.Since(start).Seconds())
			tr := store.Trade{
				Mint:        mint,
				BotName:     b.Name,
				BuySOL:      buySOL,
				SellSOL:     sellSOL,
				PnLSOL:      pnlSOL,
				Percent:     pctSOL,
				DurationSec: dur,
				ExitReason:  exitReason,
				TS:          ts,
				BuyTS:       start.Unix(),
			}
			store.ApplyStreamEntryToTrade(&tr, entry)
			store.ApplyActivitySnapshotToTrade(&tr, snap)
			saved, err := store.SaveTrade(tr)
			report.LogTradeRealtime(tr, saved, err)
			if err == nil && saved {
				_ = guard.UpdateDailyLoss(pnlSOL)
			}
			if err := report.NotifyTradeSavedWithTrade(tr); err != nil {
				log.Error("report: NotifyTradeSaved: " + err.Error())
			}
			return
		}

		time.Sleep(interval)
	}
}

func quoteStale(receivedAt time.Time, maxAgeMS int) bool {
	if maxAgeMS <= 0 || receivedAt.IsZero() {
		return false
	}
	return time.Since(receivedAt) > time.Duration(maxAgeMS)*time.Millisecond
}
