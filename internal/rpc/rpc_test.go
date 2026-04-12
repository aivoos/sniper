package rpc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	config.Load()
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
	config.Load()
	if !WaitTxConfirmed("testSig123") {
		t.Fatal("expected confirmed from RPC mock")
	}
}
