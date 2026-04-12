package recovery

import (
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
	interval := 10 * time.Second
	if config.C != nil && config.C.RecoveryInterval > 0 {
		interval = config.C.RecoveryInterval
	}
	for {
		RecoverAll()
		time.Sleep(interval)
	}
}
