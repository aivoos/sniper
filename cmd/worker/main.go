package main

import (
	"rlangga/internal/app"
	"rlangga/internal/recovery"
)

func main() {
	app.Init()

	recovery.RecoverAll()

	go recovery.StartLoop()

	app.StartWorker()
}
