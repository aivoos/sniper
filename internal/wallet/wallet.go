package wallet

import (
	"strings"

	"rlangga/internal/config"
	"rlangga/internal/rpc"
)

// BalanceHook, if set, overrides GetSOLBalance (tests only).
var BalanceHook func() float64

// GetSOLBalance returns SOL balance for the trading wallet.
// Produksi (RPC_STUB=0): getBalance ke RPC untuk PUMP_WALLET_PUBLIC_KEY.
// Tanpa config / stub / hook: fallback 1.0 SOL agar tes lokal tetap jalan.
func GetSOLBalance() float64 {
	if BalanceHook != nil {
		return BalanceHook()
	}
	cfg := config.C
	if cfg == nil || cfg.RPCStub {
		return 1.0
	}
	pk := strings.TrimSpace(cfg.WalletPublicKey)
	if pk == "" {
		return 0
	}
	sol, ok := rpc.GetSOLBalanceForPubkey(pk)
	if !ok {
		return 0
	}
	return sol
}
