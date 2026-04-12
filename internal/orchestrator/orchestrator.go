package orchestrator

import (
	"sync/atomic"

	"rlangga/internal/bot"
	"rlangga/internal/config"
)

var (
	bots []bot.BotConfig
	idx  atomic.Uint64
)

// Init installs bot profiles for round-robin assignment. Call after config.Load (uses config.C fallback when empty).
func Init(profiles []bot.BotConfig) {
	if len(profiles) == 0 {
		if config.C != nil {
			bots = []bot.BotConfig{bot.FromConfig(config.C)}
		} else {
			bots = bot.DefaultBots()
		}
		return
	}
	bots = profiles
}

// NextBot returns the next profile (round-robin, safe for concurrent callers).
func NextBot() bot.BotConfig {
	if len(bots) == 0 {
		if config.C != nil {
			return bot.FromConfig(config.C)
		}
		d := bot.DefaultBots()
		return d[0]
	}
	n := idx.Add(1)
	i := (n - 1) % uint64(len(bots))
	return bots[i]
}

// RecoveryBot labels recovery-driven sells (first profile, else FromConfig).
func RecoveryBot() bot.BotConfig {
	if len(bots) > 0 {
		return bots[0]
	}
	if config.C != nil {
		return bot.FromConfig(config.C)
	}
	return bot.DefaultBots()[0]
}
