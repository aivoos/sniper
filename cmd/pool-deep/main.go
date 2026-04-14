// Analisis mendalam per entry_pool (default: pump-amm): exit, mcap, SOL pool, panic.
// Usage:
//
//	go run ./cmd/pool-deep [pool] [start_unix end_unix]
//
// Contoh: semua histori pump-amm
//
//	go run ./cmd/pool-deep pump-amm
//
// Jendela 3h40m:
//
//	END=$(date +%s) START=$((END - 3*3600 - 40*60))
//	go run ./cmd/pool-deep pump-amm $START $END
package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

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

	var poolFilter string
	var startUnix, endUnix int64
	switch len(os.Args) {
	case 1:
		poolFilter = "pump-amm"
	case 2:
		poolFilter = os.Args[1]
	case 4:
		poolFilter = os.Args[1]
		startUnix, err = strconv.ParseInt(os.Args[2], 10, 64)
		if err != nil {
			fmt.Fprintln(os.Stderr, "start_unix:", err)
			os.Exit(1)
		}
		endUnix, err = strconv.ParseInt(os.Args[3], 10, 64)
		if err != nil {
			fmt.Fprintln(os.Stderr, "end_unix:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: pool-deep [pool] [start_unix end_unix]")
		os.Exit(1)
	}

	all, err := store.LoadAll()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var window []store.Trade
	for _, t := range all {
		if startUnix != 0 || endUnix != 0 {
			ts := t.BuyTS
			if ts == 0 {
				ts = t.TS
			}
			if ts < startUnix || ts > endUnix {
				continue
			}
		}
		window = append(window, t)
	}

	// Distribusi pool di jendela (semua pool).
	byPool := map[string]int{}
	var poolPnL map[string]float64
	poolPnL = make(map[string]float64)
	for _, t := range window {
		p := strings.TrimSpace(strings.ToLower(t.EntryPool))
		if p == "" {
			p = "(kosong)"
		}
		byPool[p]++
		poolPnL[p] += t.PnLSOL
	}
	fmt.Println("=== Distribusi entry_pool (jendela ini) ===")
	type row struct {
		pool string
		n    int
		pnl  float64
	}
	var rows []row
	for k, v := range byPool {
		rows = append(rows, row{k, v, poolPnL[k]})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].n > rows[j].n })
	for _, r := range rows {
		fmt.Printf("  %-16s n=%6d  total_pnl=%+.6f SOL\n", r.pool, r.n, r.pnl)
	}
	fmt.Println()

	poolNorm := strings.TrimSpace(strings.ToLower(poolFilter))
	var trades []store.Trade
	for _, t := range window {
		p := strings.TrimSpace(strings.ToLower(t.EntryPool))
		if p == poolNorm {
			trades = append(trades, t)
		}
	}

	winLabel := "seluruh histori"
	if startUnix != 0 {
		winLabel = fmt.Sprintf("[%d .. %d] (%ds)", startUnix, endUnix, endUnix-startUnix)
	}
	fmt.Printf("=== Analisis mendalam: pool=%q  %s ===\n", poolFilter, winLabel)
	fmt.Printf("trade_terfilter=%d\n\n", len(trades))
	if len(trades) == 0 {
		fmt.Println("Tidak ada trade dengan entry_pool yang cocok.")
		return
	}

	var wins, losses int
	var totalPnL float64
	exitAgg := map[string]struct {
		n    int
		wins int
		pnl  float64
	}{}
	for _, t := range trades {
		totalPnL += t.PnLSOL
		if t.PnLSOL > 0 {
			wins++
		} else if t.PnLSOL < 0 {
			losses++
		}
		r := t.ExitReason
		if r == "" {
			r = "(empty)"
		}
		e := exitAgg[r]
		e.n++
		e.pnl += t.PnLSOL
		if t.PnLSOL > 0 {
			e.wins++
		}
		exitAgg[r] = e
	}
	wr := 0.0
	if len(trades) > 0 {
		wr = float64(wins) / float64(len(trades)) * 100
	}
	fmt.Printf("Win rate: %.2f%%  menang=%d kalah=%d  total_pnl=%+.6f SOL\n\n", wr, wins, losses, totalPnL)

	fmt.Println("--- Exit breakdown ---")
	type exRow struct {
		reason string
		n      int
		wins   int
		pnl    float64
	}
	var exs []exRow
	for k, v := range exitAgg {
		exs = append(exs, exRow{k, v.n, v.wins, v.pnl})
	}
	sort.Slice(exs, func(i, j int) bool { return exs[i].pnl > exs[j].pnl })
	fmt.Printf("%-18s %6s %6s %8s %12s\n", "exit", "n", "wins", "win%", "total_pnl")
	for _, r := range exs {
		wp := 0.0
		if r.n > 0 {
			wp = float64(r.wins) / float64(r.n) * 100
		}
		fmt.Printf("%-18s %6d %6d %7.1f%% %12.6f\n", r.reason, r.n, r.wins, wp, r.pnl)
	}
	fmt.Println()

	// Buckets (sama ide dengan panic-profile)
	mcapEdges := []float64{10000, 15000, 20000, 25000, 30000, 40000, 50000, 75000, 100000, 200000, 500000}
	type buck struct {
		label string
		n     int
		wins  int
		pnc   int
		pnl   float64
		pncPn float64
	}
	mcapBk := make([]*buck, len(mcapEdges))
	for i, e := range mcapEdges {
		lb := ""
		if i == 0 {
			lb = fmt.Sprintf("mcap<%dk", int(e/1000))
		} else {
			lb = fmt.Sprintf("%dk-%dk", int(mcapEdges[i-1]/1000), int(e/1000))
		}
		mcapBk[i] = &buck{label: lb}
	}
	mcapOver := &buck{label: fmt.Sprintf("mcap>%dk", int(mcapEdges[len(mcapEdges)-1]/1000))}

	getMcap := func(m float64) *buck {
		if m <= 0 {
			return nil
		}
		for i, edge := range mcapEdges {
			if i == 0 && m < edge {
				return mcapBk[0]
			}
			if i > 0 && m >= mcapEdges[i-1] && m < edge {
				return mcapBk[i]
			}
		}
		return mcapOver
	}

	solEdges := []float64{50, 100, 150, 200, 300, 500, 1000}
	solBk := make([]*buck, len(solEdges))
	for i, e := range solEdges {
		lb := ""
		if i == 0 {
			lb = fmt.Sprintf("poolSol<%g", e)
		} else {
			lb = fmt.Sprintf("%g-%g", solEdges[i-1], e)
		}
		solBk[i] = &buck{label: lb}
	}
	solOver := &buck{label: fmt.Sprintf("poolSol>%g", solEdges[len(solEdges)-1])}

	getSol := func(s float64) *buck {
		if s <= 0 {
			return nil
		}
		for i, edge := range solEdges {
			if i == 0 && s < edge {
				return solBk[0]
			}
			if i > 0 && s >= solEdges[i-1] && s < edge {
				return solBk[i]
			}
		}
		return solOver
	}

	for _, t := range trades {
		isPnc := t.ExitReason == "panic"
		if b := getMcap(t.EntryMarketCapSOL); b != nil {
			b.n++
			b.pnl += t.PnLSOL
			if t.PnLSOL > 0 {
				b.wins++
			}
			if isPnc {
				b.pnc++
				b.pncPn += t.PnLSOL
			}
		}
		if b := getSol(t.EntrySolInPool); b != nil {
			b.n++
			b.pnl += t.PnLSOL
			if t.PnLSOL > 0 {
				b.wins++
			}
			if isPnc {
				b.pnc++
				b.pncPn += t.PnLSOL
			}
		}
	}

	printB := func(title string, list []*buck, over *buck) {
		fmt.Printf("--- %s ---\n", title)
		fmt.Printf("%-18s %5s %5s %5s %6s %10s %10s\n", "bucket", "n", "win", "panic", "win%", "netPnL", "panicPnL")
		for _, b := range append(list, over) {
			if b.n == 0 {
				continue
			}
			wp := float64(b.wins) / float64(b.n) * 100
			fmt.Printf("%-18s %5d %5d %5d %5.1f%% %+10.4f %+10.4f\n",
				b.label, b.n, b.wins, b.pnc, wp, b.pnl, b.pncPn)
		}
		fmt.Println()
	}
	printB("Market cap (SOL) @ entry", mcapBk, mcapOver)
	printB("SOL in pool @ entry", solBk, solOver)

	fmt.Println("--- Min mcap threshold (hanya pool terfilter) ---")
	fmt.Printf("%-14s %5s %5s %6s %12s\n", ">= mcap", "n", "win", "win%", "netPnL")
	for _, th := range []float64{10000, 15000, 20000, 25000, 30000, 40000, 50000, 75000, 100000} {
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
		if n == 0 {
			continue
		}
		wp := float64(w) / float64(n) * 100
		fmt.Printf("%-14s %5d %5d %5.1f%% %+12.6f\n", fmt.Sprintf("%.0fk", th/1000), n, w, wp, pnl)
	}
}
