package config

import (
	"os"
	"testing"
	"time"
)

var envKeys = []string{
	"TIMEOUT_MS", "RECOVERY_INTERVAL", "TRADE_SIZE", "RPC_STUB",
	"REDIS_URL", "RPC_URL", "PUMPPORTAL_URL", "PUMPAPI_URL",
	"GRACE_SECONDS", "MIN_HOLD", "MAX_HOLD", "TP_PERCENT", "SL_PERCENT",
	"PANIC_SL", "MOMENTUM_DROP", "QUOTE_INTERVAL_MS",
}

func unsetAll(t *testing.T) {
	t.Helper()
	for _, k := range envKeys {
		_ = os.Unsetenv(k)
	}
}

func TestLoad_Defaults(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	cfg := Load()
	if cfg.TimeoutMS != 1500 {
		t.Fatalf("TimeoutMS: got %d", cfg.TimeoutMS)
	}
	if cfg.RecoveryInterval != 10*time.Second {
		t.Fatalf("RecoveryInterval: got %v", cfg.RecoveryInterval)
	}
	if cfg.TradeSize != 0.1 {
		t.Fatalf("TradeSize: got %f", cfg.TradeSize)
	}
	if cfg.RPCStub {
		t.Fatal("RPCStub should be false")
	}
	if cfg.GraceSeconds != 2 || cfg.MinHold != 5 || cfg.MaxHold != 15 {
		t.Fatalf("exit defaults: grace=%d min=%d max=%d", cfg.GraceSeconds, cfg.MinHold, cfg.MaxHold)
	}
	if cfg.TakeProfit != 7 || cfg.StopLoss != 5 || cfg.PanicSL != 8 || cfg.MomentumDrop != 2.5 {
		t.Fatalf("exit thresholds: %+v", cfg)
	}
	if cfg.QuoteIntervalMS != 500 {
		t.Fatalf("QuoteIntervalMS: %d", cfg.QuoteIntervalMS)
	}
}

func TestLoad_CustomEnv(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("TIMEOUT_MS", "3000")
	t.Setenv("RECOVERY_INTERVAL", "25")
	t.Setenv("TRADE_SIZE", "0.5")
	t.Setenv("RPC_STUB", "1")
	t.Setenv("REDIS_URL", "localhost:6379")

	cfg := Load()
	if cfg.TimeoutMS != 3000 {
		t.Fatalf("TimeoutMS: got %d", cfg.TimeoutMS)
	}
	if cfg.RecoveryInterval != 25*time.Second {
		t.Fatalf("RecoveryInterval: got %v", cfg.RecoveryInterval)
	}
	if cfg.TradeSize != 0.5 {
		t.Fatalf("TradeSize: got %f", cfg.TradeSize)
	}
	if !cfg.RPCStub {
		t.Fatal("RPCStub should be true")
	}
	if cfg.RedisURL != "localhost:6379" {
		t.Fatalf("RedisURL: got %q", cfg.RedisURL)
	}
}
