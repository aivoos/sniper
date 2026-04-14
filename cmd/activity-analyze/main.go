// Analisis agregat buy/sell ratio & mcap rising pada trade terekam (Redis).
package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/joho/godotenv"

	"rlangga/internal/app"
	"rlangga/internal/store"
)

type bucket struct {
	label    string
	n        int
	wins     int
	totalPnL float64
}

func main() {
	_ = godotenv.Load()
	if err := app.Init(); err != nil {
		fmt.Println("init error:", err)
		os.Exit(1)
	}

	var trades []store.Trade
	var err error
	var label string

	switch len(os.Args) {
	case 1:
		trades, err = store.LoadTradesForReport(0)
		label = fmt.Sprintf("all trades (n=%d)", len(trades))
	case 2:
		n := 200
		if v, e := strconv.Atoi(os.Args[1]); e == nil && v > 0 {
			n = v
		}
		trades, err = store.LoadRecent(n)
		label = fmt.Sprintf("last %d trades", len(trades))
	case 3:
		start, e1 := strconv.ParseInt(os.Args[1], 10, 64)
		end, e2 := strconv.ParseInt(os.Args[2], 10, 64)
		if e1 != nil || e2 != nil {
			fmt.Fprintln(os.Stderr, "usage: activity-analyze [limit]\n       activity-analyze <start_unix> <end_unix>")
			os.Exit(1)
		}
		var all []store.Trade
		all, err = store.LoadAll()
		if err != nil {
			fmt.Println("load error:", err)
			os.Exit(1)
		}
		for _, t := range all {
			ts := t.BuyTS
			if ts == 0 {
				ts = t.TS
			}
			if ts >= start && ts <= end {
				trades = append(trades, t)
			}
		}
		label = fmt.Sprintf("window [%d..%d] (n=%d)", start, end, len(trades))
	default:
		fmt.Fprintln(os.Stderr, "usage: activity-analyze [limit]\n       activity-analyze <start_unix> <end_unix>")
		os.Exit(1)
	}

	if err != nil {
		fmt.Println("load error:", err)
		os.Exit(1)
	}
	if len(trades) == 0 {
		fmt.Println("no trades")
		return
	}

	byRatio := map[string]*bucket{}
	byMcap := map[string]*bucket{}

	for _, t := range trades {
		rk := ratioBucket(t)
		mk := mcapBucket(t)
		add(byRatio, rk, t)
		add(byMcap, mk, t)
	}

	printTable("Buy/sell ratio (entry)", label, byRatio)
	fmt.Println()
	printTable("Mcap momentum (entry)", label, byMcap)
}

func ratioBucket(t store.Trade) string {
	if !t.EntryActivityRecorded {
		return "(tanpa snapshot aktivitas)"
	}
	r := t.EntryBuySellRatio
	switch {
	case r < 1.5:
		return "ratio < 1.5"
	case r < 2.0:
		return "1.5 ≤ ratio < 2"
	case r < 3.0:
		return "2 ≤ ratio < 3"
	default:
		return "ratio ≥ 3"
	}
}

func mcapBucket(t store.Trade) string {
	if !t.EntryActivityRecorded {
		return "(tanpa snapshot aktivitas)"
	}
	if t.EntryMcapRising {
		return "mcap naik (last > prev)"
	}
	return "mcap tidak naik / datar"
}

func add(m map[string]*bucket, key string, t store.Trade) {
	a := m[key]
	if a == nil {
		a = &bucket{label: key}
		m[key] = a
	}
	a.n++
	if t.PnLSOL > 0 {
		a.wins++
	}
	a.totalPnL += t.PnLSOL
}

func printTable(title, label string, m map[string]*bucket) {
	var rows []bucket
	for _, v := range m {
		rows = append(rows, *v)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].label < rows[j].label
	})
	fmt.Printf("=== %s — %s ===\n", title, label)
	fmt.Printf("%-40s %6s %6s %7s %12s %12s\n", "bucket", "n", "wins", "win%", "total_pnl", "avg_pnl")
	for _, r := range rows {
		winPct := 0.0
		if r.n > 0 {
			winPct = float64(r.wins) / float64(r.n) * 100
		}
		avg := 0.0
		if r.n > 0 {
			avg = r.totalPnL / float64(r.n)
		}
		fmt.Printf("%-40s %6d %6d %6.1f%% %12.6f %12.6f\n", r.label, r.n, r.wins, winPct, r.totalPnL, avg)
	}
}
