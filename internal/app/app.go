package app

import (
	"errors"
	"fmt"

	"rlangga/internal/bot"
	"rlangga/internal/config"
	"rlangga/internal/executor"
	"rlangga/internal/idempotency"
	"rlangga/internal/lock"
	"rlangga/internal/log"
	"rlangga/internal/monitor"
	"rlangga/internal/orchestrator"
	"rlangga/internal/redisx"
)

// Init loads config, connects Redis, and installs multi-bot profiles (PR-004).
func Init() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if cfg.RedisURL == "" {
		return errors.New("REDIS_URL is required")
	}
	if err := redisx.Init(cfg.RedisURL); err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	profiles, err := bot.LoadBots()
	if err != nil {
		return fmt.Errorf("bots: %w", err)
	}
	orchestrator.Init(profiles)
	fmt.Println("RLANGGA INIT")
	return nil
}

// HandleMint: PR-002 adaptive monitor after successful buy (PR-001 execution path).
func HandleMint(mint string) {
	if idempotency.IsDuplicate(mint) {
		return
	}
	if !lock.LockMint(mint) {
		return
	}
	b := orchestrator.NextBot()
	log.Info(fmt.Sprintf("[%s] BUY %s", b.Name, mint))
	success := executor.BuyAndValidate(mint)
	if !success {
		lock.UnlockMint(mint)
		return
	}
	buySOL := config.C.TradeSize
	monitor.MonitorPositionWithBot(mint, buySOL, b)
	lock.UnlockMint(mint)
}

// StartWorker blocks until the process stops (event listener added in later PRs).
func StartWorker() {
	log.Info("Worker running (PR-002: adaptive exit — add mint listener / stream in follow-up)")
	select {}
}
