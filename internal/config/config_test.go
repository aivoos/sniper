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
	"TELEGRAM_BOT_TOKEN", "TELEGRAM_CHAT_ID",
	"REPORT_EVERY_N_TRADES", "REPORT_INTERVAL_MIN", "REPORT_LOAD_RECENT",
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
	// Dummy URL so runtime guard passes with default RPC_STUB=off
	t.Setenv("RPC_URL", "http://127.0.0.1:9")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
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

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
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
	if cfg.ReportEveryNTrades != 5 || cfg.ReportIntervalMin != 30 || cfg.ReportMaxTrades != 200 {
		t.Fatalf("report defaults: %+v", cfg)
	}
}

func TestLoad_RuntimeGuard_MinHoldMaxHold(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("MIN_HOLD", "20")
	t.Setenv("MAX_HOLD", "10")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when MIN_HOLD > MAX_HOLD")
	}
}

func TestLoad_InvalidInt(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("TIMEOUT_MS", "notint")
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_InvalidFloat(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("TRADE_SIZE", "x")
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_InvalidURL(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("RPC_URL", "not-a-url")
	if _, err := Load(); err == nil {
		t.Fatal("expected validate error on RPC_URL")
	}
}

func TestLoad_IntBelowMin(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("TIMEOUT_MS", "0")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for TIMEOUT_MS < 1")
	}
}

func TestLoad_TradeSizeInvalid(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("TRADE_SIZE", "0")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for TRADE_SIZE <= 0")
	}
}

func TestLoad_RPCURLRequiredWhenLiveRPC(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	if _, err := Load(); err == nil {
		t.Fatal("expected error without RPC_URL when RPC_STUB off")
	}
}

func TestLoad_RecoveryIntervalInvalid(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("RECOVERY_INTERVAL", "nan")
	if _, err := Load(); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoad_ReportEnv(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("REPORT_EVERY_N_TRADES", "10")
	t.Setenv("REPORT_INTERVAL_MIN", "15")
	t.Setenv("REPORT_LOAD_RECENT", "50")
	t.Setenv("TELEGRAM_BOT_TOKEN", "tok")
	t.Setenv("TELEGRAM_CHAT_ID", "chat1")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ReportEveryNTrades != 10 || cfg.ReportIntervalMin != 15 || cfg.ReportMaxTrades != 50 {
		t.Fatalf("%+v", cfg)
	}
	if cfg.TelegramBotToken != "tok" || cfg.TelegramChatID != "chat1" {
		t.Fatal(cfg.TelegramBotToken, cfg.TelegramChatID)
	}
}

func TestLoad_FloatNegative(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("TP_PERCENT", "-1")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for negative TP_PERCENT")
	}
}
