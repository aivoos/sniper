package config

import (
	"os"
	"strconv"
	"time"
)

// C is populated by Load and read across packages (PR-001 baseline).
var C *Config

// Config holds PR-001 environment variables.
type Config struct {
	RedisURL         string
	RPCURL           string
	PumpPortalURL    string
	PumpAPIURL       string
	TradeSize        float64
	TimeoutMS        int
	RecoveryInterval time.Duration
	// RPCStub forces confirmation without real RPC (local/dev only).
	RPCStub bool
}

// Load reads configuration from the environment. Safe to call once at startup.
func Load() *Config {
	timeout := 1500
	if v := os.Getenv("TIMEOUT_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeout = n
		}
	}
	recSec := 10
	if v := os.Getenv("RECOVERY_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			recSec = n
		}
	}
	trade := 0.1
	if v := os.Getenv("TRADE_SIZE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			trade = f
		}
	}
	stub := os.Getenv("RPC_STUB") == "1" || os.Getenv("RPC_STUB") == "true"

	C = &Config{
		RedisURL:         os.Getenv("REDIS_URL"),
		RPCURL:           os.Getenv("RPC_URL"),
		PumpPortalURL:    os.Getenv("PUMPPORTAL_URL"),
		PumpAPIURL:       os.Getenv("PUMPAPI_URL"),
		TradeSize:        trade,
		TimeoutMS:        timeout,
		RecoveryInterval: time.Duration(recSec) * time.Second,
		RPCStub:          stub,
	}
	return C
}
