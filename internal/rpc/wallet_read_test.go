package rpc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"rlangga/internal/config"
)

func TestGetSOLBalanceForPubkey_OK(t *testing.T) {
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
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	sol, ok := GetSOLBalanceForPubkey("So11111111111111111111111111111111111111112")
	if !ok || sol != 1.5 {
		t.Fatalf("got ok=%v sol=%v", ok, sol)
	}
}

func TestGetSOLBalanceForPubkey_Empty(t *testing.T) {
	if _, ok := GetSOLBalanceForPubkey("  "); ok {
		t.Fatal("expected false")
	}
}

func TestGetWalletTokens_RPC_EmptyAccounts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Method != "getTokenAccountsByOwner" {
			http.Error(w, "wrong method", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"value": []interface{}{},
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
	WalletTokensHook = nil
	t.Cleanup(func() { WalletTokensHook = nil })
	if n := len(GetWalletTokens()); n != 0 {
		t.Fatalf("got %d tokens", n)
	}
}
