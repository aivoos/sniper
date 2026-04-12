package main

import (
	"os"

	"rlangga/internal/app"
	"rlangga/internal/log"
	"rlangga/internal/recovery"
)

func main() {
	if err := app.Init(); err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	recovery.RecoverAll()

	go recovery.StartLoop()

	app.StartWorker()
}
