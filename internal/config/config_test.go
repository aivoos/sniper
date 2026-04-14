package config

import (
	"os"
	"testing"
	"time"
)

var envKeys = []string{
	"TIMEOUT_MS", "RECOVERY_INTERVAL", "TRADE_SIZE", "MAX_TRADE_SIZE_SOL", "RPC_STUB",
	"REDIS_URL", "RPC_URL", "RPC_URLS", "PUMPPORTAL_URL", "PUMPAPI_URL",
	"GRACE_SECONDS", "MIN_HOLD", "MAX_HOLD", "TP_PERCENT", "SL_PERCENT",
	"PANIC_SL", "MOMENTUM_DROP", "QUOTE_INTERVAL_MS",
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
	t.Setenv("PUMP_WALLET_PUBLIC_KEY", "So11111111111111111111111111111111111111112")
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
	if cfg.ActiveStartHour != -1 || cfg.ActiveEndHour != -1 {
		t.Fatalf("active hours default: %+v", cfg)
	}
	if cfg.FilterMaxTopHolderPct != 0 {
		t.Fatalf("FilterMaxTopHolderPct default: got %v want 0", cfg.FilterMaxTopHolderPct)
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
	if cfg.ReportEveryNTrades != 5 || cfg.ReportIntervalMin != 30 || cfg.ReportMaxTrades != 0 {
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

func TestLoad_RuntimeGuard_WSSMinMaxSOL(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("FILTER_WSS_MIN_SOL", "10")
	t.Setenv("FILTER_WSS_MAX_SOL", "5")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when FILTER_WSS_MIN_SOL > FILTER_WSS_MAX_SOL")
	}
}

func TestLoad_RuntimeGuard_WSSMinMaxMarketCap(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("FILTER_WSS_MIN_MARKET_CAP_SOL", "100")
	t.Setenv("FILTER_WSS_MAX_MARKET_CAP_SOL", "50")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when FILTER_WSS_MIN_MARKET_CAP_SOL > FILTER_WSS_MAX_MARKET_CAP_SOL")
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

func TestLoad_ActiveHoursPartialEnv(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("ACTIVE_START_HOUR", "20")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when only ACTIVE_START_HOUR set")
	}
}

func TestLoad_SimulateTrading(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("SIMULATE_TRADING", "1")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.SimulateTrading {
		t.Fatal("SimulateTrading")
	}
}

func TestLoad_SimulateEngine(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("SIMULATE_ENGINE", "1")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.SimulateEngine {
		t.Fatal("SimulateEngine")
	}
}

func TestLoad_RPC_URLs(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("RPC_URLS", "http://127.0.0.1:1, http://127.0.0.1:2")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.RPCURLs) != 2 {
		t.Fatalf("RPCURLs: %v", cfg.RPCURLs)
	}
	if cfg.RPCURL != "http://127.0.0.1:1" {
		t.Fatalf("RPCURL primary: %q", cfg.RPCURL)
	}
}

func TestLoad_RPC_URLsInvalid(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("RPC_URLS", "not-a-valid-url")
	if _, err := Load(); err == nil {
		t.Fatal("expected error on invalid RPC_URLS")
	}
}

func TestLoad_RPC_URLsEmptyAfterComma(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("RPC_URLS", " , , ")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when RPC_URLS has no valid URLs")
	}
}

func TestLoad_SimulateSynthTuning(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("SIMULATE_SYNTH_AMPLITUDE_PCT", "18")
	t.Setenv("SIMULATE_SYNTH_PERIOD_SEC", "8")
	t.Setenv("SIMULATE_SYNTH_DRIFT_PCT", "-1")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SimulateSynthAmplitudePct != 18 || cfg.SimulateSynthPeriodSec != 8 || cfg.SimulateSynthDriftPct != -1 {
		t.Fatalf("synth tuning: %+v", cfg)
	}
}

func TestLoad_PaperTrade_HeliosMainnetFromKey(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "0")
	t.Setenv("PUMPPORTAL_URL", "http://127.0.0.1:9")
	t.Setenv("PUMP_WALLET_PUBLIC_KEY", "So11111111111111111111111111111111111111112")
	t.Setenv("PAPER_TRADE", "1")
	t.Setenv("HELIUS_API_KEY", "test-key")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.PaperTrade {
		t.Fatal("PaperTrade")
	}
	if cfg.RPCURL != "https://mainnet.helius-rpc.com/?api-key=test-key" {
		t.Fatalf("RPCURL: %q", cfg.RPCURL)
	}
}

func TestLoad_PaperTrade_RPCURLExplicitWins(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "0")
	t.Setenv("PUMPPORTAL_URL", "http://127.0.0.1:9")
	t.Setenv("PUMP_WALLET_PUBLIC_KEY", "So11111111111111111111111111111111111111112")
	t.Setenv("PAPER_TRADE", "1")
	t.Setenv("RPC_URL", "https://example.com/rpc")
	t.Setenv("HELIUS_API_KEY", "ignored")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RPCURL != "https://example.com/rpc" {
		t.Fatalf("want explicit RPC_URL, got %q", cfg.RPCURL)
	}
}

func TestLoad_RealTradingRequiresWalletPubkey(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "0")
	t.Setenv("RPC_URL", "http://127.0.0.1:9")
	t.Setenv("PUMPPORTAL_URL", "http://127.0.0.1:9")
	if _, err := Load(); err == nil {
		t.Fatal("expected error without PUMP_WALLET_PUBLIC_KEY when ENABLE_TRADING=true")
	}
}

func TestLoad_PaperTrade_WithRPCStubFails(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("PUMPPORTAL_URL", "http://127.0.0.1:9")
	t.Setenv("PAPER_TRADE", "1")
	t.Setenv("HELIUS_API_KEY", "k")
	if _, err := Load(); err == nil {
		t.Fatal("expected error: PAPER_TRADE needs RPC_STUB=0")
	}
}

func TestLoad_ActiveHoursOK(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("TZ", "Asia/Jakarta")
	t.Setenv("ACTIVE_START_HOUR", "20")
	t.Setenv("ACTIVE_END_HOUR", "2")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TZ != "Asia/Jakarta" || cfg.ActiveStartHour != 20 || cfg.ActiveEndHour != 2 {
		t.Fatalf("%+v", cfg)
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

func TestLoad_GuardDefaults(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxDailyLoss != 0 || cfg.MinBalance != 0 || !cfg.EnableTrading || cfg.MaxDailyTrades != 0 {
		t.Fatalf("%+v", cfg)
	}
}

func TestLoad_EnableTradingTrueAliases(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("ENABLE_TRADING", "yes")
	cfg, err := Load()
	if err != nil || !cfg.EnableTrading {
		t.Fatalf("%v %+v", err, cfg)
	}
}

func TestLoad_EnableTradingUnknownUsesDefault(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("RPC_URL", "http://127.0.0.1:9")
	t.Setenv("ENABLE_TRADING", "maybe")
	cfg, err := Load()
	if err != nil || !cfg.EnableTrading {
		t.Fatalf("%v %+v", err, cfg)
	}
}

func TestLoad_PumpWSFields(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("RPC_URL", "http://127.0.0.1:9")
	t.Setenv("PUMP_WS_URL", "wss://pumpportal.fun/api/data")
	t.Setenv("PUMP_WS_SUBSCRIBE_JSON", `[{"method":"subscribeNewToken"}]`)
	t.Setenv("PUMP_WS_AUTO_HANDLE", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PumpWSURL == "" || !cfg.PumpWSAutoHandle || cfg.PumpWSSubscribeJSON == "" {
		t.Fatalf("%+v", cfg)
	}
}

func TestLoad_GuardCustom(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("MAX_DAILY_LOSS", "1.5")
	t.Setenv("MIN_BALANCE", "0.05")
	t.Setenv("ENABLE_TRADING", "false")
	t.Setenv("MAX_DAILY_TRADES", "10")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MaxDailyLoss != 1.5 || cfg.MinBalance != 0.05 || cfg.EnableTrading || cfg.MaxDailyTrades != 10 {
		t.Fatalf("%+v", cfg)
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

func TestLoad_HazardDefaults(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("RPC_URL", "http://127.0.0.1:9")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LockTTLMin != 12 || cfg.QuoteMaxAgeMS != 0 || cfg.MinDust != 0 || cfg.StaleBalanceWaitMS != 0 {
		t.Fatalf("%+v", cfg)
	}
}

func TestLoad_HazardCustom(t *testing.T) {
	unsetAll(t)
	t.Cleanup(func() { unsetAll(t) })
	t.Setenv("RPC_STUB", "1")
	t.Setenv("RPC_URL", "http://127.0.0.1:9")
	t.Setenv("MIN_DUST", "0.0001")
	t.Setenv("QUOTE_MAX_AGE_MS", "750")
	t.Setenv("LOCK_TTL_MIN", "15")
	t.Setenv("STALE_BALANCE_WAIT_MS", "300")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MinDust != 0.0001 || cfg.QuoteMaxAgeMS != 750 || cfg.LockTTLMin != 15 || cfg.StaleBalanceWaitMS != 300 {
		t.Fatalf("%+v", cfg)
	}
}
