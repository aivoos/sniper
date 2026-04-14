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

type agg struct {
	reason   string
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
			fmt.Fprintln(os.Stderr, "usage: exit-reasons [limit]\n       exit-reasons <start_unix> <end_unix>")
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
		fmt.Fprintln(os.Stderr, "usage: exit-reasons [limit]\n       exit-reasons <start_unix> <end_unix>")
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

	m := map[string]*agg{}
	for _, t := range trades {
		r := t.ExitReason
		if r == "" {
			r = "(empty)"
		}
		a := m[r]
		if a == nil {
			a = &agg{reason: r}
			m[r] = a
		}
		a.n++
		if t.PnLSOL > 0 {
			a.wins++
		}
		a.totalPnL += t.PnLSOL
	}

	var rows []agg
	for _, v := range m {
		rows = append(rows, *v)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].totalPnL == rows[j].totalPnL {
			return rows[i].n > rows[j].n
		}
		return rows[i].totalPnL > rows[j].totalPnL
	})

	fmt.Printf("=== Exit reasons — %s ===\n", label)
	fmt.Printf("%-22s %6s %6s %7s %12s %12s\n", "reason", "n", "wins", "win%", "total_pnl", "avg_pnl")
	for _, r := range rows {
		winPct := 0.0
		if r.n > 0 {
			winPct = float64(r.wins) / float64(r.n) * 100
		}
		avg := r.totalPnL / float64(r.n)
		fmt.Printf("%-22s %6d %6d %6.1f%% %12.6f %12.6f\n", r.reason, r.n, r.wins, winPct, r.totalPnL, avg)
	}
}
