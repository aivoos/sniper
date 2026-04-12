package app

import (
	"fmt"
	"os"
	"time"

	"rlangga/internal/config"
	"rlangga/internal/executor"
	"rlangga/internal/idempotency"
	"rlangga/internal/lock"
	"rlangga/internal/log"
	"rlangga/internal/redisx"
)

// Init loads config and connects Redis. Exits process on fatal error.
func Init() {
	cfg := config.Load()
	if cfg.RedisURL == "" {
		log.Error("REDIS_URL is required")
		os.Exit(1)
	}
	if err := redisx.Init(cfg.RedisURL); err != nil {
		log.Error("redis: " + err.Error())
		os.Exit(1)
	}
	fmt.Println("RLANGGA INIT")
}

// HandleMint is PR-001 baseline: buy → hold → sell (replaced by PR-002 monitor).
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
	time.Sleep(10 * time.Second)
	executor.SafeSellWithValidation(mint)
	lock.UnlockMint(mint)
}

// StartWorker blocks until the process stops (event listener added in later PRs).
func StartWorker() {
	log.Info("Worker running (PR-001: no mint listener — integrate pump/stream in follow-up)")
	select {}
}
