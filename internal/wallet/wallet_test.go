package wallet

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"rlangga/internal/config"
)

func TestGetTradeSize_MaxCap(t *testing.T) {
	old := config.C
	t.Cleanup(func() { config.C = old })
	BalanceHook = func() float64 { return 100 }
	t.Cleanup(func() { BalanceHook = nil })
	config.C = &config.Config{
		TradeSizePct:    10,
		MaxTradeSizeSOL: 2,
	}
	if s := GetTradeSize(); s != 2 {
		t.Fatalf("want cap 2 SOL, got %v", s)
	}
}

func TestGetSOLBalance_Default(t *testing.T) {
	old := config.C
	t.Cleanup(func() { config.C = old })
	config.C = nil
	BalanceHook = nil
	if GetSOLBalance() != 5.0 {
		t.Fatal(GetSOLBalance())
	}
}

func TestGetSOLBalance_Hook(t *testing.T) {
	BalanceHook = func() float64 { return 3.14 }
	t.Cleanup(func() { BalanceHook = nil })
	if GetSOLBalance() != 3.14 {
		t.Fatal(GetSOLBalance())
	}
}

func TestGetSOLBalance_SimulateEngineLiveBalance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"context": map[string]interface{}{"slot": 1},
				"value":   uint64(1_500_000_000),
			},
		})
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RPC_URL", srv.URL)
	t.Setenv("RPC_STUB", "0")
	t.Setenv("ENABLE_TRADING", "false")
	t.Setenv("TIMEOUT_MS", "2000")
	t.Setenv("PUMP_WALLET_PUBLIC_KEY", "So11111111111111111111111111111111111111112")
	t.Setenv("SIMULATE_ENGINE", "1")
	t.Setenv("SIMULATE_USE_LIVE_BALANCE", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	BalanceHook = nil
	t.Cleanup(func() { config.C = nil })
	if b := GetSOLBalance(); b != 1.5 {
		t.Fatalf("got %v want 1.5 SOL (live balance under simulate)", b)
	}
}

func TestGetSOLBalance_FromRPC(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"context": map[string]interface{}{"slot": 1},
				"value":   uint64(2_000_000_000),
			},
		})
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RPC_URL", srv.URL)
	t.Setenv("RPC_STUB", "0")
	t.Setenv("ENABLE_TRADING", "false")
	t.Setenv("TIMEOUT_MS", "2000")
	t.Setenv("PUMP_WALLET_PUBLIC_KEY", "So11111111111111111111111111111111111111112")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	BalanceHook = nil
	t.Cleanup(func() { BalanceHook = nil })
	if b := GetSOLBalance(); b != 2 {
		t.Fatalf("got %v want 2 SOL", b)
	}
}
