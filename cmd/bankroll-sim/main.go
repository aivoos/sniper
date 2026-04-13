// Simulasi modal (bankroll) dari trade tertutup di Redis.
// Fokus: bagaimana pnl_sol mempengaruhi modal awal (mis. 5 SOL), dan analisis per bucket entry_market_cap_sol.
package main

import (
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/joho/godotenv"

	"rlangga/internal/redisx"
	"rlangga/internal/store"
)

type bucket struct {
	name    string
	min     float64
	max     float64
	n       int
	wins    int
	totalPn float64
}

func main() {
	_ = godotenv.Load()

	startCapital := float64FromEnv("START_CAPITAL_SOL", 5.0)
	nTrades := intFromEnv("N_TRADES", 200)
	minMcap := float64FromEnv("MIN_ENTRY_MCAP_SOL", 0)

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

	trades, err := store.LoadRecent(nTrades)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(trades) == 0 {
		fmt.Println("Tidak ada trade di Redis.")
		return
	}

	// Urutkan kronologis lama → baru untuk equity curve.
	sort.Slice(trades, func(i, j int) bool {
		ti := trades[i].BuyTS
		if ti == 0 {
			ti = trades[i].TS
		}
		tj := trades[j].BuyTS
		if tj == 0 {
			tj = trades[j].TS
		}
		return ti < tj
	})

	eq := startCapital
	peak := eq
	maxDD := 0.0
	minEq := eq

	filtered := 0
	skippedNoMcap := 0

	for _, t := range trades {
		mcap := t.EntryMarketCapSOL
		if minMcap > 0 {
			if mcap <= 0 {
				skippedNoMcap++
				continue
			}
			if mcap < minMcap {
				continue
			}
		}
		filtered++
		eq += t.PnLSOL
		if eq > peak {
			peak = eq
		}
		if eq < minEq {
			minEq = eq
		}
		if peak > 0 {
			dd := (peak - eq) / peak
			if dd > maxDD {
				maxDD = dd
			}
		}
	}

	now := time.Now().Format(time.RFC3339)
	fmt.Println("=== Bankroll sim (Redis trades:list) ===")
	fmt.Println("time:", now)
	fmt.Printf("start_capital_sol=%.6f n_trades_loaded=%d n_trades_used=%d\n", startCapital, len(trades), filtered)
	if minMcap > 0 {
		fmt.Printf("min_entry_mcap_sol=%.2f skipped_no_mcap=%d\n", minMcap, skippedNoMcap)
	}
	ret := 0.0
	if startCapital > 0 {
		ret = (eq - startCapital) / startCapital * 100
	}
	fmt.Printf("end_capital_sol=%.6f return_pct=%.2f%%\n", eq, ret)
	fmt.Printf("min_equity_sol=%.6f max_drawdown_pct=%.2f%%\n\n", minEq, maxDD*100)

	// Bucket market cap entry (hanya trade yang punya mcap > 0).
	bs := []bucket{
		{name: "mcap_0_50", min: 0, max: 50},
		{name: "mcap_50_200", min: 50, max: 200},
		{name: "mcap_200_1k", min: 200, max: 1000},
		{name: "mcap_1k_10k", min: 1000, max: 10000},
		{name: "mcap_ge_10k", min: 10000, max: math.Inf(1)},
	}

	usedForBucket := 0
	for _, t := range trades {
		mcap := t.EntryMarketCapSOL
		if mcap <= 0 {
			continue
		}
		usedForBucket++
		for i := range bs {
			if mcap >= bs[i].min && mcap < bs[i].max {
				bs[i].n++
				bs[i].totalPn += t.PnLSOL
				if t.PnLSOL > 0 {
					bs[i].wins++
				}
				break
			}
		}
	}

	fmt.Printf("--- entry_market_cap_sol buckets (n_with_mcap=%d) ---\n", usedForBucket)
	fmt.Printf("%-12s %6s %6s %6s %12s %12s\n", "bucket", "n", "win", "win%", "total_pnl", "avg_pnl")
	for _, b := range bs {
		if b.n == 0 {
			continue
		}
		wr := float64(b.wins) / float64(b.n) * 100
		avg := b.totalPn / float64(b.n)
		fmt.Printf("%-12s %6d %6d %5.1f%% %12.6f %12.6f\n", b.name, b.n, b.wins, wr, b.totalPn, avg)
	}
}

func float64FromEnv(k string, def float64) float64 {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

func intFromEnv(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
