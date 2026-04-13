package recovery

import (
	"context"
	"time"

	"rlangga/internal/config"
	"rlangga/internal/executor"
	"rlangga/internal/guard"
	"rlangga/internal/orchestrator"
	"rlangga/internal/quote"
	"rlangga/internal/redisx"
	"rlangga/internal/report"
	"rlangga/internal/rpc"
	"rlangga/internal/sellguard"
	"rlangga/internal/store"
)

// RecoverAll scans wallet tokens and force-sells any with balance > 0.
func RecoverAll() {
	tokens := rpc.GetWalletTokens()
	for _, t := range tokens {
		if t.Amount <= 0 {
			continue
		}
		cfg := config.C
		if cfg != nil && cfg.MinDust > 0 && t.Amount < cfg.MinDust {
			continue
		}
		if !sellguard.TryAcquireSellExit(t.Mint) {
			continue
		}
		if !executor.SafeSellWithValidation(t.Mint) {
			sellguard.ReleaseSellExit(t.Mint)
			continue
		}
		if cfg == nil || redisx.Client == nil {
			sellguard.ReleaseSellExit(t.Mint)
			continue
		}
		sellSOL := quote.GetSellQuote(t.Mint)
		buySOL := cfg.TradeSize
		ts := time.Now().Unix()
		pnlSOL := sellSOL - buySOL
		pct := 0.0
		if buySOL > 0 {
			pct = (pnlSOL / buySOL) * 100
		}
		rb := orchestrator.RecoveryBot()
		tr := store.Trade{
			Mint:        t.Mint,
			BotName:     rb.Name,
			BuySOL:      buySOL,
			SellSOL:     sellSOL,
			PnLSOL:      pnlSOL,
			Percent:     pct,
			DurationSec: 0,
			ExitReason:  "recovery",
			TS:          ts,
			BuyTS:       ts,
		}
		saved, err := store.SaveTrade(tr)
		report.LogTradeRealtime(tr, saved, err)
		if err == nil && saved {
			_ = guard.UpdateDailyLoss(pnlSOL)
		}
		_ = report.NotifyTradeSaved()
		if cfg.StaleBalanceWaitMS > 0 {
			time.Sleep(time.Duration(cfg.StaleBalanceWaitMS) * time.Millisecond)
		}
		sellguard.ReleaseSellExit(t.Mint)
	}
}

// StartLoop runs RecoverAll forever with RECOVERY_INTERVAL between iterations.
func StartLoop() {
	startLoop(context.Background())
}

func startLoop(ctx context.Context) {
	interval := 10 * time.Second
	if config.C != nil && config.C.RecoveryInterval > 0 {
		interval = config.C.RecoveryInterval
	}
	for {
		RecoverAll()
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}
