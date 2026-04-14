package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/joho/godotenv"

	"rlangga/internal/config"
	"rlangga/internal/redisx"
	"rlangga/internal/store"
)

func main() {
	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if err := redisx.Init(cfg.RedisURL); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	trades, err := store.LoadRecent(2000)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Printf("=== Deep MCap + SOL-in-Pool Analysis (%d trades) ===\n\n", len(trades))

	// Granular mcap buckets
	type bucket struct {
		label    string
		total    int
		wins     int
		panics   int
		pnl      float64
		panicPnL float64
		winPnL   float64
	}

	mcapEdges := []float64{10000, 15000, 20000, 25000, 30000, 40000, 50000, 75000, 100000, 200000, 500000}
	mcapBuckets := make([]*bucket, len(mcapEdges))
	for i, e := range mcapEdges {
		label := ""
		if i == 0 {
			label = fmt.Sprintf("<%.0fk", e/1000)
		} else {
			label = fmt.Sprintf("%.0fk-%.0fk", mcapEdges[i-1]/1000, e/1000)
		}
		mcapBuckets[i] = &bucket{label: label}
	}
	mcapOver := &bucket{label: fmt.Sprintf(">%.0fk", mcapEdges[len(mcapEdges)-1]/1000)}

	getMcapBucket := func(mcap float64) *bucket {
		if mcap <= 0 {
			return nil
		}
		for i, edge := range mcapEdges {
			if i == 0 && mcap < edge {
				return mcapBuckets[0]
			}
			if i > 0 && mcap >= mcapEdges[i-1] && mcap < edge {
				return mcapBuckets[i]
			}
		}
		return mcapOver
	}

	// SOL in pool granular
	solEdges := []float64{50, 100, 150, 200, 300, 500, 1000}
	solBuckets := make([]*bucket, len(solEdges))
	for i, e := range solEdges {
		label := ""
		if i == 0 {
			label = fmt.Sprintf("<%.0f", e)
		} else {
			label = fmt.Sprintf("%.0f-%.0f", solEdges[i-1], e)
		}
		solBuckets[i] = &bucket{label: label}
	}
	solOver := &bucket{label: fmt.Sprintf(">%.0f", solEdges[len(solEdges)-1])}

	getSolBucket := func(sol float64) *bucket {
		if sol <= 0 {
			return nil
		}
		for i, edge := range solEdges {
			if i == 0 && sol < edge {
				return solBuckets[0]
			}
			if i > 0 && sol >= solEdges[i-1] && sol < edge {
				return solBuckets[i]
			}
		}
		return solOver
	}

	for _, t := range trades {
		isPanic := t.ExitReason == "panic"
		isWin := t.PnLSOL > 0

		if b := getMcapBucket(t.EntryMarketCapSOL); b != nil {
			b.total++
			b.pnl += t.PnLSOL
			if isPanic {
				b.panics++
				b.panicPnL += t.PnLSOL
			}
			if isWin {
				b.wins++
				b.winPnL += t.PnLSOL
			}
		}

		if b := getSolBucket(t.EntrySolInPool); b != nil {
			b.total++
			b.pnl += t.PnLSOL
			if isPanic {
				b.panics++
				b.panicPnL += t.PnLSOL
			}
			if isWin {
				b.wins++
				b.winPnL += t.PnLSOL
			}
		}
	}

	printBuckets := func(title string, bkts []*bucket, extra *bucket) {
		fmt.Printf("--- %s ---\n", title)
		fmt.Printf("%-15s %5s %5s %5s %6s %10s %10s\n", "Bucket", "Total", "Win", "Pnc", "Win%", "NetPnL", "PanicPnL")
		all := append(bkts, extra)
		for _, b := range all {
			if b.total == 0 {
				continue
			}
			wPct := float64(b.wins) / float64(b.total) * 100
			fmt.Printf("%-15s %5d %5d %5d %5.1f%% %+10.4f %+10.4f\n",
				b.label, b.total, b.wins, b.panics, wPct, b.pnl, b.panicPnL)
		}
		fmt.Println()
	}

	printBuckets("Market Cap SOL (granular)", mcapBuckets, mcapOver)
	printBuckets("SOL in Pool (granular)", solBuckets, solOver)

	// Best mcap sweet spot: cumulative from top
	fmt.Println("--- MCap Threshold Simulator (filter >= X) ---")
	fmt.Printf("%-12s %5s %5s %6s %10s\n", "Min MCap", "Trd", "Win", "Win%", "NetPnL")
	thresholds := []float64{10000, 12000, 15000, 18000, 20000, 25000, 30000, 40000, 50000}
	for _, th := range thresholds {
		n, w, pnl := 0, 0, 0.0
		for _, t := range trades {
			if t.EntryMarketCapSOL >= th {
				n++
				pnl += t.PnLSOL
				if t.PnLSOL > 0 {
					w++
				}
			}
		}
		wr := 0.0
		if n > 0 {
			wr = float64(w) / float64(n) * 100
		}
		fmt.Printf("%-12s %5d %5d %5.1f%% %+10.4f\n", fmt.Sprintf(">= %.0fk", th/1000), n, w, wr, pnl)
	}

	fmt.Println()

	// SOL in pool threshold
	fmt.Println("--- SOL-in-Pool Threshold Simulator (filter >= X) ---")
	fmt.Printf("%-12s %5s %5s %6s %10s\n", "Min SOL", "Trd", "Win", "Win%", "NetPnL")
	solThresh := []float64{50, 100, 150, 200, 300, 500}
	for _, th := range solThresh {
		n, w, pnl := 0, 0, 0.0
		for _, t := range trades {
			if t.EntrySolInPool >= th {
				n++
				pnl += t.PnLSOL
				if t.PnLSOL > 0 {
					w++
				}
			}
		}
		wr := 0.0
		if n > 0 {
			wr = float64(w) / float64(n) * 100
		}
		fmt.Printf("%-12s %5d %5d %5.1f%% %+10.4f\n", fmt.Sprintf(">= %.0f", th), n, w, wr, pnl)
	}

	// Top PnL trades — what mcap?
	fmt.Println("\n--- Top 10 Best Trades (by PnL) ---")
	sort.Slice(trades, func(i, j int) bool { return trades[i].PnLSOL > trades[j].PnLSOL })
	for i := 0; i < 10 && i < len(trades); i++ {
		t := trades[i]
		fmt.Printf("%+.4f SOL  mcap=%.0f  sol_pool=%.1f  exit=%s  mint=%s\n",
			t.PnLSOL, t.EntryMarketCapSOL, t.EntrySolInPool, t.ExitReason, t.Mint[:8]+".."+t.Mint[len(t.Mint)-4:])
	}

	fmt.Println("\n--- Top 10 Worst Trades (by PnL) ---")
	sort.Slice(trades, func(i, j int) bool { return trades[i].PnLSOL < trades[j].PnLSOL })
	for i := 0; i < 10 && i < len(trades); i++ {
		t := trades[i]
		fmt.Printf("%+.4f SOL  mcap=%.0f  sol_pool=%.1f  exit=%s  mint=%s\n",
			t.PnLSOL, t.EntryMarketCapSOL, t.EntrySolInPool, t.ExitReason, t.Mint[:8]+".."+t.Mint[len(t.Mint)-4:])
	}
}
