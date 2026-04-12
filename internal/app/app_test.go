package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"rlangga/internal/config"
	"rlangga/internal/redisx"
)

func TestInit_OK(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if redisx.Client != nil {
			_ = redisx.Client.Close()
		}
		redisx.Client = nil
		config.C = nil
		s.Close()
	})

	t.Setenv("REDIS_URL", s.Addr())
	t.Setenv("RPC_STUB", "1")
	unsetConfigEnv(t)
	if err := Init(); err != nil {
		t.Fatal(err)
	}
}

func TestInit_ErrMissingRedisURL(t *testing.T) {
	t.Cleanup(func() { config.C = nil })
	t.Setenv("RPC_STUB", "1")
	unsetConfigEnv(t)
	t.Setenv("REDIS_URL", "")
	if err := Init(); err == nil {
		t.Fatal("expected error without REDIS_URL")
	}
}

func TestInit_ConfigError(t *testing.T) {
	t.Cleanup(func() { config.C = nil })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("TIMEOUT_MS", "notanint")
	unsetConfigEnv(t)
	if err := Init(); err == nil {
		t.Fatal("expected config error")
	}
}

func TestInit_ErrRedisDial(t *testing.T) {
	t.Cleanup(func() {
		redisx.Client = nil
		config.C = nil
	})
	t.Setenv("REDIS_URL", "127.0.0.1:59999")
	t.Setenv("RPC_STUB", "1")
	unsetConfigEnv(t)
	if err := Init(); err == nil {
		t.Fatal("expected redis dial error")
	}
}

func TestStartWorker_Runs(t *testing.T) {
	go StartWorker()
	time.Sleep(20 * time.Millisecond)
}

func TestHandleMint_AdaptiveExit(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	portal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/buy":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"signature": "buy-sig"})
		case "/quote":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]float64{"sol": 0})
		case "/sell":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"signature": "sell-sig"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(func() {
		if redisx.Client != nil {
			_ = redisx.Client.Close()
		}
		redisx.Client = nil
		config.C = nil
		s.Close()
		portal.Close()
	})

	t.Setenv("REDIS_URL", s.Addr())
	t.Setenv("PUMPPORTAL_URL", portal.URL)
	t.Setenv("PUMPAPI_URL", portal.URL)
	t.Setenv("RPC_STUB", "1")
	t.Setenv("QUOTE_INTERVAL_MS", "20")
	t.Setenv("PANIC_SL", "8")
	unsetConfigEnv(t)

	if err := Init(); err != nil {
		t.Fatal(err)
	}

	mint := "So11111111111111111111111111111111111111112"
	done := make(chan struct{})
	go func() {
		HandleMint(mint)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("HandleMint did not finish")
	}

	// Second call: idempotency short-circuit
	HandleMint(mint)
}

func TestHandleMint_BuyFails(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if redisx.Client != nil {
			_ = redisx.Client.Close()
		}
		redisx.Client = nil
		config.C = nil
		s.Close()
	})
	t.Setenv("REDIS_URL", s.Addr())
	t.Setenv("PUMPPORTAL_URL", "")
	t.Setenv("PUMPAPI_URL", "")
	t.Setenv("RPC_STUB", "1")
	unsetConfigEnv(t)
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	HandleMint("mintBuyFail")
}

func TestHandleMint_NoRedisLock(t *testing.T) {
	t.Cleanup(func() { config.C = nil })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("RPC_URL", "http://127.0.0.1:9")
	unsetConfigEnv(t)
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	redisx.Client = nil
	HandleMint("noRedis")
}

func unsetConfigEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"TIMEOUT_MS", "RECOVERY_INTERVAL", "TRADE_SIZE", "PUMPPORTAL_URL", "PUMPAPI_URL",
		"GRACE_SECONDS", "MIN_HOLD", "MAX_HOLD", "TP_PERCENT", "SL_PERCENT",
		"PANIC_SL", "MOMENTUM_DROP", "QUOTE_INTERVAL_MS", "RPC_URL",
		"TELEGRAM_BOT_TOKEN", "TELEGRAM_CHAT_ID",
		"REPORT_EVERY_N_TRADES", "REPORT_INTERVAL_MIN", "REPORT_LOAD_RECENT",
	}
	for _, k := range keys {
		_ = os.Unsetenv(k)
	}
}
