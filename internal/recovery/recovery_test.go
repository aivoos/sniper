package recovery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"rlangga/internal/config"
	"rlangga/internal/rpc"
	"rlangga/internal/testutil"
)

func TestRecoverAll_SellHTTPFails(t *testing.T) {
	testutil.UseMiniredis(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PUMPPORTAL_URL", srv.URL)
	t.Setenv("RPC_STUB", "1")
	t.Setenv("RPC_URL", "http://127.0.0.1:9")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	rpc.WalletTokensHook = func() []rpc.Token {
		return []rpc.Token{{Mint: "bad", Amount: 1}}
	}
	t.Cleanup(func() { rpc.WalletTokensHook = nil })
	RecoverAll()
}

func TestRecoverAll_NoPanic(t *testing.T) {
	rpc.WalletTokensHook = nil
	t.Cleanup(func() { rpc.WalletTokensHook = nil })
	RecoverAll()
}

func TestRecoverAll_SellsPositiveBalance(t *testing.T) {
	testutil.UseMiniredis(t)
	sellSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sell":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"signature": "x"})
		case "/quote":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]float64{"sol": 0.11})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(sellSrv.Close)

	t.Setenv("PUMPPORTAL_URL", sellSrv.URL)
	t.Setenv("RPC_STUB", "1")
	t.Setenv("RPC_URL", "http://127.0.0.1:9")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}

	rpc.WalletTokensHook = func() []rpc.Token {
		return []rpc.Token{{Mint: "mintZ", Amount: 1}}
	}
	t.Cleanup(func() { rpc.WalletTokensHook = nil })

	RecoverAll()
}

func TestStartLoop_Cancel(t *testing.T) {
	t.Cleanup(func() { config.C = nil })
	for _, k := range []string{
		"TIMEOUT_MS", "RECOVERY_INTERVAL", "TRADE_SIZE", "RPC_STUB", "RPC_URL",
		"PUMPPORTAL_URL", "PUMPAPI_URL", "GRACE_SECONDS", "MIN_HOLD", "MAX_HOLD",
		"TP_PERCENT", "SL_PERCENT", "PANIC_SL", "MOMENTUM_DROP", "QUOTE_INTERVAL_MS", "REDIS_URL",
	} {
		_ = os.Unsetenv(k)
	}
	t.Setenv("RPC_STUB", "1")
	t.Setenv("RECOVERY_INTERVAL", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		startLoop(ctx)
		close(done)
	}()

	time.Sleep(15 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("startLoop did not stop")
	}
}
