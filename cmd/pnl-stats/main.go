// Command ringkas: agregat PnL dari Redis (trades:list). Muat .env dari cwd.
package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"

	"rlangga/internal/aggregate"
	"rlangga/internal/redisx"
	"rlangga/internal/store"
)

func main() {
	_ = godotenv.Load()
	addr := os.Getenv("REDIS_URL")
	if addr == "" {
		fmt.Fprintln(os.Stderr, "REDIS_URL wajib (sama seperti worker).")
		os.Exit(1)
	}
	if err := redisx.Init(addr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer func() { _ = redisx.Client.Close() }()

	n := 10000
	if len(os.Args) > 1 {
		if _, err := fmt.Sscanf(os.Args[1], "%d", &n); err != nil || n <= 0 {
			n = 10000
		}
	}
	trades, err := store.LoadRecent(n)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	st := aggregate.ComputeStats(trades)
	streak := aggregate.LossStreak(trades)
	fmt.Println("=== PnL (Redis trades:list, terbaru dulu) ===")
	fmt.Printf("trade_tertutup=%d menang=%d kalah=%d winrate=%.2f%%\n", st.Total, st.Win, st.Loss, st.Winrate)
	fmt.Printf("total_pnl_sol=%.6f rata_pnl_sol=%.6f\n", st.TotalPnL, st.AvgPnL)
	fmt.Printf("loss_streak_terbaru=%d\n", streak)
}
