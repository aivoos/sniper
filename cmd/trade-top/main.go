package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"

	"rlangga/internal/app"
	"rlangga/internal/store"
)

func main() {
	_ = godotenv.Load()
	if err := app.Init(); err != nil {
		fmt.Println("init error:", err)
		os.Exit(1)
	}

	n := 200
	if len(os.Args) > 1 {
		if v, err := strconv.Atoi(os.Args[1]); err == nil && v > 0 {
			n = v
		}
	}

	trades, err := store.LoadRecent(n)
	if err != nil {
		fmt.Println("load error:", err)
		os.Exit(1)
	}
	if len(trades) == 0 {
		fmt.Println("no trades")
		return
	}

	best := trades[0]
	worst := trades[0]
	for _, t := range trades[1:] {
		if t.PnLSOL > best.PnLSOL {
			best = t
		}
		if t.PnLSOL < worst.PnLSOL {
			worst = t
		}
	}

	fmt.Printf("=== Top trade (last %d) ===\n", len(trades))
	fmt.Printf("pnl_sol=%.6f pct=%.2f%% mint=%s bot=%s exit=%s buy_ts=%d sell_ts=%d\n",
		best.PnLSOL, best.Percent, best.Mint, best.BotName, best.ExitReason, best.BuyTS, best.TS)
	fmt.Printf("=== Worst trade (last %d) ===\n", len(trades))
	fmt.Printf("pnl_sol=%.6f pct=%.2f%% mint=%s bot=%s exit=%s buy_ts=%d sell_ts=%d\n",
		worst.PnLSOL, worst.Percent, worst.Mint, worst.BotName, worst.ExitReason, worst.BuyTS, worst.TS)
}
