package wallet

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"rlangga/internal/config"
)

func TestGetSOLBalance_Default(t *testing.T) {
	old := config.C
	t.Cleanup(func() { config.C = old })
	config.C = nil
	BalanceHook = nil
	if GetSOLBalance() != 1.0 {
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
