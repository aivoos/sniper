package wallet

import (
	"math"
	"strings"

	"rlangga/internal/config"
	"rlangga/internal/rpc"
)

// BalanceHook, if set, overrides GetSOLBalance (tests only).
var BalanceHook func() float64

// GetSOLBalance returns SOL balance for the trading wallet.
// Produksi (RPC_STUB=0, tanpa SIMULATE_ENGINE): getBalance ke RPC untuk PUMP_WALLET_PUBLIC_KEY.
// SIMULATE_ENGINE: default saldo virtual 5 SOL; jika SIMULATE_USE_LIVE_BALANCE=1 dan RPC ok → saldo nyata (guard & TRADE_SIZE_PCT).
// RPC_STUB: selalu 5 SOL virtual.
func GetSOLBalance() float64 {
	if BalanceHook != nil {
		return BalanceHook()
	}
	cfg := config.C
	if cfg == nil {
		return 5.0
	}
	if cfg.RPCStub {
		return 5.0
	}
	if cfg.SimulateEngine {
		if cfg.SimulateUseLiveBalance {
			pk := strings.TrimSpace(cfg.WalletPublicKey)
			if pk != "" {
				if sol, ok := rpc.GetSOLBalanceForPubkey(pk); ok {
					return sol
				}
			}
		}
		return 5.0
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

// GetTradeSize returns the SOL amount for a single trade.
// If TRADE_SIZE_PCT > 0, calculates from wallet balance; otherwise uses static TRADE_SIZE.
// MAX_TRADE_SIZE_SOL (jika > 0) memotong hasil agar satu BUY tidak terlalu besar untuk pasar.
// Result is rounded to 4 decimal places (0.0001 SOL precision).
func GetTradeSize() float64 {
	cfg := config.C
	if cfg == nil {
		return 0.1
	}
	var size float64
	if cfg.TradeSizePct > 0 {
		bal := GetSOLBalance()
		size = bal * cfg.TradeSizePct / 100
		size = math.Round(size*10000) / 10000
		if size < 0.001 {
			return 0
		}
	} else {
		size = cfg.TradeSize
	}
	if cfg.MaxTradeSizeSOL > 0 && size > cfg.MaxTradeSizeSOL {
		size = math.Round(cfg.MaxTradeSizeSOL*10000) / 10000
	}
	return size
}
