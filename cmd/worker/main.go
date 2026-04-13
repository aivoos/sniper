package main

import (
	"os"

	"github.com/joho/godotenv"

	"rlangga/internal/app"
	"rlangga/internal/log"
	"rlangga/internal/recovery"
)

func main() {
	// Muat .env dari direktori kerja (opsional; variabel shell tetap diutamakan).
	_ = godotenv.Load()

	if err := app.Init(); err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	recovery.RecoverAll()

	go recovery.StartLoop()

	app.StartWorker()
}
