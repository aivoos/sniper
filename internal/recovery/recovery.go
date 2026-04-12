package recovery

import (
	"context"
	"time"

	"rlangga/internal/config"
	"rlangga/internal/executor"
	"rlangga/internal/quote"
	"rlangga/internal/redisx"
	"rlangga/internal/report"
	"rlangga/internal/rpc"
	"rlangga/internal/store"
)

// RecoverAll scans wallet tokens and force-sells any with balance > 0.
func RecoverAll() {
	tokens := rpc.GetWalletTokens()
	for _, t := range tokens {
		if t.Amount <= 0 {
			continue
		}
		if !executor.SafeSellWithValidation(t.Mint) {
			continue
		}
		cfg := config.C
		if cfg == nil || redisx.Client == nil {
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
		_ = store.SaveTrade(store.Trade{
			Mint:        t.Mint,
			BuySOL:      buySOL,
			SellSOL:     sellSOL,
			PnLSOL:      pnlSOL,
			Percent:     pct,
			DurationSec: 0,
			TS:          ts,
		})
		_ = report.NotifyTradeSaved()
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
