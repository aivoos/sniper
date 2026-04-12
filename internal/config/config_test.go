package config

import (
	"os"
	"testing"
	"time"
)

var envKeys = []string{
	"TIMEOUT_MS", "RECOVERY_INTERVAL", "TRADE_SIZE", "RPC_STUB",
	"REDIS_URL", "RPC_URL", "PUMPPORTAL_URL", "PUMPAPI_URL",
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
