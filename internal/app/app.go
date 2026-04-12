package app

import (
	"errors"
	"fmt"

	"rlangga/internal/config"
	"rlangga/internal/executor"
	"rlangga/internal/idempotency"
	"rlangga/internal/lock"
	"rlangga/internal/log"
	"rlangga/internal/monitor"
	"rlangga/internal/redisx"
)

// Init loads config and connects Redis.
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
	success := executor.BuyAndValidate(mint)
	if !success {
		lock.UnlockMint(mint)
		return
	}
	buySOL := config.C.TradeSize
	monitor.MonitorPosition(mint, buySOL)
	lock.UnlockMint(mint)
}

// StartWorker blocks until the process stops (event listener added in later PRs).
func StartWorker() {
	log.Info("Worker running (PR-002: adaptive exit — add mint listener / stream in follow-up)")
	select {}
}
