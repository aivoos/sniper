package bot

import (
	"encoding/json"
	"fmt"
	"os"

	"rlangga/internal/config"
)

// BotConfig is exit/threshold profile for one logical bot (PR-004).
type BotConfig struct {
	Name         string  `json:"name"`
	MinHold      int     `json:"min_hold"`
	MaxHold      int     `json:"max_hold"`
	TakeProfit   float64 `json:"take_profit"`
	StopLoss     float64 `json:"stop_loss"`
	PanicLoss    float64 `json:"panic_loss"`
	MomentumDrop float64 `json:"momentum_drop"`
	GraceSeconds int     `json:"grace_seconds"`
	GraceSL        float64 `json:"grace_sl"`
	GraceTP        float64 `json:"grace_tp"`
	GraceTrailDrop float64 `json:"grace_trail_drop"`
}

// FromConfig maps global env config to a single BotConfig (fallback / tests).
func FromConfig(c *config.Config) BotConfig {
	if c == nil {
		return BotConfig{Name: "default"}
	}
	return BotConfig{
		Name:         "default",
		MinHold:      c.MinHold,
		MaxHold:      c.MaxHold,
		TakeProfit:   c.TakeProfit,
		StopLoss:     c.StopLoss,
		PanicLoss:    c.PanicSL,
		MomentumDrop: c.MomentumDrop,
		GraceSeconds: c.GraceSeconds,
		GraceSL:        c.GraceSL,
		GraceTP:        c.GraceTP,
		GraceTrailDrop: c.GraceTrailDrop,
	}
}

// DefaultBots returns the documented baseline pair when BOTS_JSON is unset.
func DefaultBots() []BotConfig {
	return []BotConfig{
		{
			Name: "bot-10s", MinHold: 5, MaxHold: 10,
			TakeProfit: 7, StopLoss: 5, PanicLoss: 8, MomentumDrop: 2.5, GraceSeconds: 2,
		},
		{
			Name: "bot-15s", MinHold: 5, MaxHold: 15,
			TakeProfit: 8, StopLoss: 5, PanicLoss: 8, MomentumDrop: 3, GraceSeconds: 2,
		},
	}
}

// LoadBots reads BOTS_JSON or returns DefaultBots. Validates MinHold <= MaxHold per profile.
func LoadBots() ([]BotConfig, error) {
	raw := os.Getenv("BOTS_JSON")
	if raw == "" {
		return DefaultBots(), nil
	}
	var out []BotConfig
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("BOTS_JSON: %w", err)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("BOTS_JSON: empty array")
	}
	for i := range out {
		if out[i].Name == "" {
			out[i].Name = fmt.Sprintf("bot-%d", i)
		}
		if out[i].MinHold > out[i].MaxHold {
			return nil, fmt.Errorf("bot %q: min_hold > max_hold", out[i].Name)
		}
	}
	return out, nil
}
