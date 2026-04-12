package rpc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"rlangga/internal/config"
)

func TestWaitTxConfirmed_EmptySig(t *testing.T) {
	if WaitTxConfirmed("") {
		t.Fatal("empty sig must be false")
	}
}

func TestWaitTxConfirmed_RPCStub(t *testing.T) {
	t.Setenv("RPC_STUB", "1")
	t.Setenv("RPC_URL", "")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if !WaitTxConfirmed("any-sig") {
		t.Fatal("stub should confirm non-empty sig")
	}
}

func TestWaitTxConfirmed_RPCFinalized(t *testing.T) {
	t.Setenv("RPC_STUB", "0")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"context": map[string]interface{}{"slot": 1},
				"value": []interface{}{
					map[string]interface{}{"confirmationStatus": "finalized"},
				},
			},
		})
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RPC_URL", srv.URL)
	t.Setenv("TIMEOUT_MS", "2000")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if !WaitTxConfirmed("testSig123") {
		t.Fatal("expected confirmed from RPC mock")
	}
}

func TestGetWalletTokens_Default(t *testing.T) {
	WalletTokensHook = nil
	toks := GetWalletTokens()
	if len(toks) != 0 {
		t.Fatalf("got %v", toks)
	}
}

func TestGetWalletTokens_Hook(t *testing.T) {
	WalletTokensHook = func() []Token {
		return []Token{{Mint: "a", Amount: 1}}
	}
	t.Cleanup(func() { WalletTokensHook = nil })
	toks := GetWalletTokens()
	if len(toks) != 1 || toks[0].Mint != "a" {
		t.Fatalf("got %+v", toks)
	}
}

func TestGetTxStatus_ConnectionFailed(t *testing.T) {
	t.Setenv("RPC_STUB", "0")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	u := srv.URL
	srv.Close()
	t.Setenv("RPC_URL", u)
	t.Setenv("TIMEOUT_MS", "500")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if getTxStatus("sig") != "unknown" {
		t.Fatal("expected unknown on connection error")
	}
}

func TestGetTxStatus_BadJSON(t *testing.T) {
	t.Setenv("RPC_STUB", "0")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not-json`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RPC_URL", srv.URL)
	t.Setenv("TIMEOUT_MS", "500")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if getTxStatus("sig") != "unknown" {
		t.Fatal("expected unknown on bad JSON")
	}
}

func TestGetTxStatus_NilConfirmationStatus(t *testing.T) {
	t.Setenv("RPC_STUB", "0")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{
				"value": []interface{}{
					map[string]interface{}{},
				},
			},
		})
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RPC_URL", srv.URL)
	t.Setenv("TIMEOUT_MS", "500")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if getTxStatus("sig") != "unknown" {
		t.Fatal("expected unknown")
	}
}

func TestGetTxStatus_EmptyValue(t *testing.T) {
	t.Setenv("RPC_STUB", "0")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{"value": []interface{}{}},
		})
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RPC_URL", srv.URL)
	t.Setenv("TIMEOUT_MS", "500")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if getTxStatus("sig") != "unknown" {
		t.Fatal("expected unknown")
	}
}

func TestWaitTxConfirmed_PollsUntilConfirmed(t *testing.T) {
	t.Setenv("RPC_STUB", "0")
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := n.Add(1)
		st := "processed"
		if c >= 2 {
			st = "confirmed"
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{
				"value": []interface{}{
					map[string]interface{}{"confirmationStatus": st},
				},
			},
		})
	}))
	t.Cleanup(srv.Close)
	t.Setenv("RPC_URL", srv.URL)
	t.Setenv("TIMEOUT_MS", "5000")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if !WaitTxConfirmed("sig") {
		t.Fatal("expected confirmed after poll")
	}
}
