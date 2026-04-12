package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
)

// C is populated by Load and read across packages (PR-001 + PR-002).
var C *Config

var validate = validator.New()

// Config holds environment for worker, exit engine, and recovery.
type Config struct {
	// Env-backed (see rlangga-env-contract.md). RedisURL checked at app.Init (worker), not here — tests load partial env.
	RedisURL      string `validate:"omitempty"`
	RPCURL        string `validate:"omitempty,url"`
	PumpPortalURL string `validate:"omitempty,url"`
	PumpAPIURL    string `validate:"omitempty,url"`

	TradeSize        float64       `validate:"gt=0"`
	TimeoutMS        int           `validate:"gt=0"`
	RecoveryInterval time.Duration `validate:"gt=0"`
	RPCStub          bool

	GraceSeconds    int     `validate:"gte=0"`
	MinHold         int     `validate:"gt=0"`
	MaxHold         int     `validate:"gt=0"`
	TakeProfit      float64 `validate:"gte=0"`
	StopLoss        float64 `validate:"gte=0"`
	PanicSL         float64 `validate:"gte=0"`
	MomentumDrop    float64 `validate:"gte=0"`
	QuoteIntervalMS int     `validate:"gt=0"`
}

// Load reads configuration from the environment, validates struct tags, then applies runtime guards.
// Sets global C on success. Safe to call once at startup (worker); tests may call repeatedly with t.Setenv.
func Load() (*Config, error) {
	c, err := parseEnv()
	if err != nil {
		return nil, err
	}
	if err := validate.Struct(c); err != nil {
		return nil, fmt.Errorf("config validate: %w", err)
	}
	if err := validateRuntimeGuards(c); err != nil {
		return nil, err
	}
	C = c
	return c, nil
}

func parseEnv() (*Config, error) {
	timeout, err := intFromEnv("TIMEOUT_MS", 1500, 1)
	if err != nil {
		return nil, err
	}
	recSec, err := intFromEnv("RECOVERY_INTERVAL", 10, 1)
	if err != nil {
		return nil, err
	}
	trade, err := floatFromEnv("TRADE_SIZE", 0.1, 0, false)
	if err != nil {
		return nil, err
	}

	grace, err := intFromEnv("GRACE_SECONDS", 2, 0)
	if err != nil {
		return nil, err
	}
	minHold, err := intFromEnv("MIN_HOLD", 5, 1)
	if err != nil {
		return nil, err
	}
	maxHold, err := intFromEnv("MAX_HOLD", 15, 1)
	if err != nil {
		return nil, err
	}
	tp, err := floatFromEnv("TP_PERCENT", 7, 0, true)
	if err != nil {
		return nil, err
	}
	sl, err := floatFromEnv("SL_PERCENT", 5, 0, true)
	if err != nil {
		return nil, err
	}
	panicSL, err := floatFromEnv("PANIC_SL", 8, 0, true)
	if err != nil {
		return nil, err
	}
	mom, err := floatFromEnv("MOMENTUM_DROP", 2.5, 0, true)
	if err != nil {
		return nil, err
	}
	qms, err := intFromEnv("QUOTE_INTERVAL_MS", 500, 1)
	if err != nil {
		return nil, err
	}

	stub := os.Getenv("RPC_STUB") == "1" || os.Getenv("RPC_STUB") == "true"

	c := &Config{
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
	return c, nil
}

// intFromEnv parses int; empty → default; min is minimum allowed (inclusive).
func intFromEnv(key string, def, min int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	if n < min {
		return 0, fmt.Errorf("%s: must be >= %d", key, min)
	}
	return n, nil
}

// floatFromEnv parses float64; empty → default. allowZero uses min 0 for non-empty values.
func floatFromEnv(key string, def float64, min float64, allowZero bool) (float64, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	if allowZero {
		if f < min {
			return 0, fmt.Errorf("%s: must be >= %v", key, min)
		}
	} else if f <= min {
		return 0, fmt.Errorf("%s: must be > %v", key, min)
	}
	return f, nil
}

// validateRuntimeGuards encodes cross-field and operational rules (validator tags are per-field only).
func validateRuntimeGuards(c *Config) error {
	if c.MinHold > c.MaxHold {
		return fmt.Errorf("config runtime: MIN_HOLD (%d) must be <= MAX_HOLD (%d)", c.MinHold, c.MaxHold)
	}
	if !c.RPCStub && c.RPCURL == "" {
		return fmt.Errorf("config runtime: RPC_URL is required when RPC_STUB is off")
	}
	return nil
}
