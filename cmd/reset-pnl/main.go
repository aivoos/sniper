package main

import (
	"context"
	"fmt"
	"os"

	"github.com/joho/godotenv"

	"rlangga/internal/guard"
	"rlangga/internal/positionlimit"
	"rlangga/internal/redisx"
	"rlangga/internal/report"
	"rlangga/internal/store"
)

func main() {
	_ = godotenv.Load()
	addr := os.Getenv("REDIS_URL")
	if addr == "" {
		fmt.Fprintln(os.Stderr, "REDIS_URL wajib di-set (sama seperti worker).")
		os.Exit(1)
	}
	if err := redisx.Init(addr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx := context.Background()
	if err := store.ClearTradesAndDedupe(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "store:", err)
		os.Exit(1)
	}
	if err := report.ResetReportState(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "report:", err)
		os.Exit(1)
	}
	if err := guard.ResetDailyStats(); err != nil {
		fmt.Fprintln(os.Stderr, "guard:", err)
		os.Exit(1)
	}
	if err := positionlimit.ResetRedisState(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "positionlimit:", err)
		os.Exit(1)
	}
	fmt.Println("OK: riwayat trade + dedupe, counter laporan, stat harian guard, dan rlangga:pos:* (slot posisi) sudah direset.")
}
