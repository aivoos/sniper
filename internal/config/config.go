package config

import (
	"os"
	"strconv"
	"time"
)

// C is populated by Load and read across packages (PR-001 + PR-002).
var C *Config

// Config holds environment for worker, exit engine, and recovery.
type Config struct {
	RedisURL         string
	RPCURL           string
	PumpPortalURL    string
	PumpAPIURL       string
	TradeSize        float64
	TimeoutMS        int
	RecoveryInterval time.Duration
	RPCStub          bool

	// PR-002 adaptive exit (percents / seconds — see rlangga-env-contract.md)
	GraceSeconds    int
	MinHold         int
	MaxHold         int
	TakeProfit      float64
	StopLoss        float64
	PanicSL         float64
	MomentumDrop    float64
	QuoteIntervalMS int
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

	grace := getInt("GRACE_SECONDS", 2)
	minHold := getInt("MIN_HOLD", 5)
	maxHold := getInt("MAX_HOLD", 15)
	tp := getFloat("TP_PERCENT", 7)
	sl := getFloat("SL_PERCENT", 5)
	panicSL := getFloat("PANIC_SL", 8)
	mom := getFloat("MOMENTUM_DROP", 2.5)
	qms := getInt("QUOTE_INTERVAL_MS", 500)

	C = &Config{
		RedisURL:         os.Getenv("REDIS_URL"),
		RPCURL:           os.Getenv("RPC_URL"),
		PumpPortalURL:    os.Getenv("PUMPPORTAL_URL"),
		PumpAPIURL:       os.Getenv("PUMPAPI_URL"),
		TradeSize:        trade,
		TimeoutMS:        timeout,
		RecoveryInterval: time.Duration(recSec) * time.Second,
		RPCStub:          stub,
		GraceSeconds:     grace,
		MinHold:          minHold,
		MaxHold:          maxHold,
		TakeProfit:       tp,
		StopLoss:         sl,
		PanicSL:          panicSL,
		MomentumDrop:     mom,
		QuoteIntervalMS:  qms,
	}
	return C
}

func getInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return def
	}
	return n
}

func getFloat(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}
