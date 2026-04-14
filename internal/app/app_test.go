package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"rlangga/internal/config"
	"rlangga/internal/guard"
	"rlangga/internal/pumpws"
	"rlangga/internal/redisx"
	"rlangga/internal/wallet"
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

func TestInit_BotsJSONInvalid(t *testing.T) {
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
	t.Setenv("BOTS_JSON", `{`)
	if err := Init(); err == nil {
		t.Fatal("expected bots parse error")
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

func TestStartPumpStream_NilConfig(t *testing.T) {
	startPumpStream(context.Background(), nil)
}

func TestStartPumpStream_EmptyWSURL(t *testing.T) {
	startPumpStream(context.Background(), &config.Config{})
}

func TestEntrySnapshotPasses_ExtendedFields(t *testing.T) {
	cfg := &config.Config{
		FilterMinEntrySolInPool:         10,
		FilterMinBurnedLiquidityPct:     100,
		FilterRejectPoolCreatedByCustom: true,
	}

	// Custom pool harus ditolak.
	ev := &pumpws.StreamEvent{
		PoolCreatedBy:      "custom",
		SolInPool:          100,
		HasSolInPool:       true,
		BurnedLiquidityPct: 100,
		HasBurnedLiquidity: true,
		MarketCapSOL:       2000,
		HasMarketCapSOL:    true,
	}
	if entrySnapshotPasses(cfg, ev) {
		t.Fatal("expected custom pool rejected")
	}

	// Migrated pool + burned 100% + solInPool cukup: lolos.
	ev.PoolCreatedBy = "pump"
	if !entrySnapshotPasses(cfg, ev) {
		t.Fatal("expected pass for pump pool")
	}

	// Burned liquidity kurang: ditolak.
	ev.BurnedLiquidityPct = 99
	if entrySnapshotPasses(cfg, ev) {
		t.Fatal("expected burned liquidity rejected")
	}
}

func TestHandleMint_SimulateEngine(t *testing.T) {
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
	unsetConfigEnv(t)
	t.Setenv("REDIS_URL", s.Addr())
	t.Setenv("RPC_STUB", "1")
	t.Setenv("SIMULATE_ENGINE", "1")
	t.Setenv("PUMPPORTAL_URL", "")
	t.Setenv("PUMPAPI_URL", "")
	t.Setenv("QUOTE_INTERVAL_MS", "20")
	t.Setenv("MAX_HOLD", "1")
	t.Setenv("MIN_HOLD", "1")
	t.Setenv("GRACE_SECONDS", "0")
	t.Setenv("PANIC_SL", "8")
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	mint := "So11111111111111111111111111111111111111112"
	done := make(chan struct{})
	go func() {
		HandleMint(mint, nil)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("HandleMint simulate engine did not finish")
	}
}

func TestHandleMint_SimulateTrading(t *testing.T) {
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
	unsetConfigEnv(t)
	t.Setenv("REDIS_URL", s.Addr())
	t.Setenv("RPC_STUB", "1")
	t.Setenv("SIMULATE_TRADING", "1")
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	HandleMint("So11111111111111111111111111111111111111112", nil)
	HandleMint("So11111111111111111111111111111111111111112", nil)
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

	// unsetConfigEnv menghapus kunci yang kita butuhkan — panggil dulu, lalu Setenv.
	unsetConfigEnv(t)
	t.Setenv("REDIS_URL", s.Addr())
	t.Setenv("PUMPPORTAL_URL", portal.URL)
	t.Setenv("PUMPAPI_URL", portal.URL)
	t.Setenv("RPC_STUB", "1")
	t.Setenv("QUOTE_INTERVAL_MS", "20")
	t.Setenv("PANIC_SL", "8")

	if err := Init(); err != nil {
		t.Fatal(err)
	}

	mint := "So11111111111111111111111111111111111111112"
	done := make(chan struct{})
	go func() {
		HandleMint(mint, nil)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("HandleMint did not finish")
	}

	// Second call: idempotency short-circuit
	HandleMint(mint, nil)
}

func TestHandleMint_GuardDailyQuota(t *testing.T) {
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
	unsetConfigEnv(t)
	t.Setenv("REDIS_URL", s.Addr())
	t.Setenv("RPC_STUB", "1")
	t.Setenv("MAX_DAILY_TRADES", "1")
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	if err := guard.IncrDailyTradeCount(); err != nil {
		t.Fatal(err)
	}
	HandleMint("mintQuotaBlock", nil)
}

func TestHandleMint_GuardKillSwitch(t *testing.T) {
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
	unsetConfigEnv(t)
	t.Setenv("REDIS_URL", s.Addr())
	t.Setenv("RPC_STUB", "1")
	t.Setenv("MAX_DAILY_LOSS", "0.1")
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	if err := guard.UpdateDailyLoss(-0.2); err != nil {
		t.Fatal(err)
	}
	HandleMint("mintKillSwitch", nil)
}

func TestHandleMint_GuardBlocksLowBalance(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		wallet.BalanceHook = nil
		if redisx.Client != nil {
			_ = redisx.Client.Close()
		}
		redisx.Client = nil
		config.C = nil
		s.Close()
	})
	unsetConfigEnv(t)
	t.Setenv("REDIS_URL", s.Addr())
	t.Setenv("RPC_STUB", "1")
	t.Setenv("MIN_BALANCE", "5")
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	wallet.BalanceHook = func() float64 { return 0.01 }
	HandleMint("mintGuardBlock", nil)
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
	unsetConfigEnv(t)
	t.Setenv("REDIS_URL", s.Addr())
	t.Setenv("PUMPPORTAL_URL", "")
	t.Setenv("PUMPAPI_URL", "")
	t.Setenv("RPC_STUB", "1")
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	HandleMint("mintBuyFail", nil)
}

func TestHandleMint_NoRedisLock(t *testing.T) {
	t.Cleanup(func() { config.C = nil })
	unsetConfigEnv(t)
	t.Setenv("RPC_STUB", "1")
	t.Setenv("RPC_URL", "http://127.0.0.1:9")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	redisx.Client = nil
	HandleMint("noRedis", nil)
}

func unsetConfigEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"TIMEOUT_MS", "RECOVERY_INTERVAL", "TRADE_SIZE", "MAX_TRADE_SIZE_SOL", "PUMPPORTAL_URL", "PUMPAPI_URL",
		"GRACE_SECONDS", "MIN_HOLD", "MAX_HOLD", "TP_PERCENT", "SL_PERCENT",
		"PANIC_SL", "MOMENTUM_DROP", "QUOTE_INTERVAL_MS", "RPC_URL", "RPC_URLS",
		"TELEGRAM_BOT_TOKEN", "TELEGRAM_CHAT_ID",
		"REPORT_EVERY_N_TRADES", "REPORT_INTERVAL_MIN", "REPORT_LOAD_RECENT",
		"BOTS_JSON",
		"MAX_DAILY_LOSS", "MIN_BALANCE", "ENABLE_TRADING", "MAX_DAILY_TRADES",
		"MIN_DUST", "QUOTE_MAX_AGE_MS", "LOCK_TTL_MIN", "STALE_BALANCE_WAIT_MS",
		"PUMP_WS_URL", "PUMP_WS_SUBSCRIBE_JSON", "PUMP_WS_FALLBACK_URL", "PUMP_WS_FALLBACK_SUBSCRIBE_JSON", "PUMP_WS_AUTO_HANDLE",
		"TZ", "ACTIVE_START_HOUR", "ACTIVE_END_HOUR",
		"PUMP_NATIVE", "PUMPPORTAL_API_KEY", "PUMPPORTAL_QUOTE_URL", "PUMPAPI_QUOTE_URL",
		"PUMP_WALLET_PUBLIC_KEY", "PUMP_PRIVATE_KEY", "PUMP_SLIPPAGE", "PUMPAPI_SLIPPAGE", "PUMP_PRIORITY_FEE",
		"PUMPAPI_QUOTE_MINT", "PUMPAPI_POOL_ID", "PUMPAPI_GUARANTEED_DELIVERY", "PUMPAPI_JITO_TIP",
		"PUMPAPI_MAX_PRIORITY_FEE", "PUMPAPI_PRIORITY_FEE_MODE",
		"PAPER_TRADE", "HELIUS_API_KEY",
		"SIMULATE_TRADING", "SIMULATE_ENGINE", "SIMULATE_USE_LIVE_BALANCE",
		"SIMULATE_SYNTH_AMPLITUDE_PCT", "SIMULATE_SYNTH_PERIOD_SEC", "SIMULATE_SYNTH_DRIFT_PCT",
		"FILTER_ANTI_RUG", "FILTER_REJECT_FREEZE_AUTHORITY", "FILTER_REJECT_MINT_AUTHORITY",
		"FILTER_MAX_TOP_HOLDER_PCT", "FILTER_RPC_FAIL_OPEN", "FILTER_REQUIRE_INITIAL_BUY",
		"FILTER_WSS_ALLOW_TX_TYPES", "FILTER_WSS_DENY_TX_TYPES", "FILTER_WSS_ALLOW_METHODS",
		"FILTER_WSS_MIN_SOL", "FILTER_WSS_MAX_SOL",
		"FILTER_WSS_POOL", "FILTER_WSS_MIN_MARKET_CAP_SOL", "FILTER_WSS_MAX_MARKET_CAP_SOL",
		"TRADE_SQLITE_PATH", "FILTER_MIN_INITIAL_BUY", "FILTER_MIN_ENTRY_MARKET_CAP_SOL",
		"FILTER_MIN_ENTRY_SOL_IN_POOL", "FILTER_MIN_BURNED_LIQUIDITY_PCT", "FILTER_REJECT_POOL_CREATED_BY_CUSTOM",
	}
	for _, k := range keys {
		_ = os.Unsetenv(k)
	}
}
