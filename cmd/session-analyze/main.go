// Analisis trade Redis dalam rentang waktu [start_unix, end_unix] (buy_ts), agregat per entry_pool.
package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/joho/godotenv"

	"rlangga/internal/redisx"
	"rlangga/internal/store"
)

type poolAgg struct {
	pool     string
	n        int
	wins     int
	totalPnL float64
	ibN      int
	ibSum    float64
	mcapN    int
	mcapSum  float64
}

func main() {
	_ = godotenv.Load()
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: session-analyze <start_unix> <end_unix>")
		fmt.Fprintln(os.Stderr, "  filter trade dengan buy_ts di [start,end] (inklusif)")
		os.Exit(1)
	}
	start, err := strconv.ParseInt(os.Args[1], 10, 64)
	if err != nil {
		fmt.Fprintln(os.Stderr, "start_unix:", err)
		os.Exit(1)
	}
	end, err := strconv.ParseInt(os.Args[2], 10, 64)
	if err != nil {
		fmt.Fprintln(os.Stderr, "end_unix:", err)
		os.Exit(1)
	}
	addr := os.Getenv("REDIS_URL")
	if addr == "" {
		fmt.Fprintln(os.Stderr, "REDIS_URL wajib")
		os.Exit(1)
	}
	if err := redisx.Init(addr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer func() { _ = redisx.Client.Close() }()

	// Muat seluruh histori Redis lalu filter ke jendela waktu (bukan cap 200).
	trades, err := store.LoadAll()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	byPool := make(map[string]*poolAgg)
	var sessN int
	var sessPnL float64

	for _, t := range trades {
		ts := t.BuyTS
		if ts == 0 {
			ts = t.TS
		}
		if ts < start || ts > end {
			continue
		}
		sessN++
		sessPnL += t.PnLSOL
		p := strings.TrimSpace(t.EntryPool)
		if p == "" {
			p = "(tanpa_snapshot_pool)"
		}
		a, ok := byPool[p]
		if !ok {
			a = &poolAgg{pool: p}
			byPool[p] = a
		}
		a.n++
		a.totalPnL += t.PnLSOL
		if t.PnLSOL > 0 {
			a.wins++
		}
		if t.EntryInitialBuy > 0 {
			a.ibN++
			a.ibSum += t.EntryInitialBuy
		}
		if t.EntryMarketCapSOL > 0 {
			a.mcapN++
			a.mcapSum += t.EntryMarketCapSOL
		}
	}

	fmt.Printf("=== Sesi [%d .. %d] unix (durasi %ds) ===\n", start, end, end-start)
	fmt.Printf("trade_di_jendela=%d total_pnl_sol=%.6f\n\n", sessN, sessPnL)

	if sessN == 0 {
		fmt.Println("Tidak ada trade di jendela waktu ini (cek filter WSS / AUTO_HANDLE / sinyal).")
		return
	}

	type row struct {
		pool string
		*poolAgg
	}
	var rows []row
	for k, v := range byPool {
		rows = append(rows, row{pool: k, poolAgg: v})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].totalPnL > rows[j].totalPnL })

	fmt.Println("--- Per pool (entry_pool dari WSS saat entry) ---")
	fmt.Printf("%-20s %5s %5s %6s %12s %10s %12s %14s\n",
		"pool", "n", "menang", "win%", "total_pnl", "rata_pnl", "avg_mcap", "avg_initial_buy")
	for _, r := range rows {
		wr := 0.0
		if r.n > 0 {
			wr = float64(r.wins) / float64(r.n) * 100
		}
		avg := r.totalPnL / float64(r.n)
		avgMcap := 0.0
		if r.mcapN > 0 {
			avgMcap = r.mcapSum / float64(r.mcapN)
		}
		avgIB := 0.0
		if r.ibN > 0 {
			avgIB = r.ibSum / float64(r.ibN)
		}
		fmt.Printf("%-20s %5d %5d %5.1f%% %12.6f %10.6f %12.2f %14.3g\n",
			r.pool, r.n, r.wins, wr, r.totalPnL, avg, avgMcap, avgIB)
	}

	fmt.Println("\n--- Rekomendasi (heuristik; sample kecil bisa noise) ---")
	for _, r := range rows {
		if r.n < 2 {
			continue
		}
		if r.totalPnL > 0 && r.wins*2 >= r.n {
			fmt.Printf("Prioritas pertimbangkan FILTER_WSS_POOL termasuk: %s (n=%d, total_pnl=%.6f)\n", r.pool, r.n, r.totalPnL)
		}
	}
	for i := len(rows) - 1; i >= 0; i-- {
		r := rows[i]
		if r.n < 2 {
			continue
		}
		if r.totalPnL < 0 || r.wins == 0 {
			fmt.Printf("Pertimbangkan kecualikan / hati-hati: %s (n=%d, total_pnl=%.6f)\n", r.pool, r.n, r.totalPnL)
		}
	}
}
