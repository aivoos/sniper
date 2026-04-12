package recovery

import (
	"context"
	"time"

	"rlangga/internal/config"
	"rlangga/internal/executor"
	"rlangga/internal/rpc"
)

// RecoverAll scans wallet tokens and force-sells any with balance > 0.
func RecoverAll() {
	tokens := rpc.GetWalletTokens()
	for _, t := range tokens {
		if t.Amount > 0 {
			executor.SafeSellWithValidation(t.Mint)
		}
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
