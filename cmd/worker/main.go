package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"rlangga/internal/app"
	"rlangga/internal/log"
	"rlangga/internal/recovery"
	"rlangga/internal/report"
)

func main() {
	_ = godotenv.Load()

	if err := app.Init(); err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Info("shutdown: received " + sig.String())
		cancel()
	}()

	recovery.RecoverAll()
	go recovery.StartLoopWithContext(ctx)

	app.StartWorkerWithContext(ctx)

	log.Info("shutdown: sending bye report to telegram...")
	_ = report.SendPlainMessage("BOT STOPPED (graceful shutdown)")
	log.Info("shutdown: done")
}
