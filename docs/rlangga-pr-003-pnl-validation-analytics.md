# PR-003 — PnL & validasi (data + metrik + laporan)

**ID:** PR-003  
**Jenjang dokumen:** spesifikasi deliverable — turunan dari [rlangga-blueprint-v2.md](./rlangga-blueprint-v2.md)  
**Prasyarat:** [PR-001](./rlangga-pr-001-core-engine-recovery-validation.md) (sell terkonfirmasi), [PR-002](./rlangga-pr-002-adaptive-exit-pnl.md) (monitor / quote / durasi posisi)

**Objektif**

| Target | Keterangan |
|--------|------------|
| Semua trade tercatat | Append-only di Redis |
| PnL per trade akurat | Basis SOL (buy vs sell / estimate final) |
| Metrik | Win rate, rata-rata PnL, total |
| Loss streak | Sinyal risiko (market jelek / overtrade) |
| Laporan | Ringkasan Telegram |
| Landasan produk | Tuning parameter TP/SL dan keputusan scale |

---

## 1. Ruang lingkup (final)

| Area | Termasuk |
|------|----------|
| Trade store | Redis, append-only |
| Aggregation | `ComputeStats` |
| Metrik | Win rate, avg, total PnL SOL |
| Loss streak | Urutan trade terbaru |
| Ringkasan | Telegram (`SendSummary` / `MaybeSendSummary`) |
| Integrasi | Pasca-SELL sukses di alur monitor (PR-002) |

---

## 2. Modul

```text
internal/
  store/
  aggregate/
  report/
```

---

## 3. Model data (terkunci)

### Kunci Redis

| Kunci | Operasi | Keterangan |
|-------|---------|------------|
| `trades:list` | `LPUSH` (prepend trade baru) | Log append-only |
| `stats:daily_loss` | (nanti) | Dipakai PR-005 |

### Rekaman trade (kontrak JSON)

```json
{
  "mint": "xxx",
  "buy_sol": 0.1,
  "sell_sol": 0.107,
  "pnl_sol": 0.007,
  "percent": 7.0,
  "duration_sec": 8,
  "ts": 1710000000
}
```

---

## 4. Menyimpan trade (append-only)

```go
type Trade struct {
    Mint        string  `json:"mint"`
    BuySOL      float64 `json:"buy_sol"`
    SellSOL     float64 `json:"sell_sol"`
    PnLSOL      float64 `json:"pnl_sol"`
    Percent     float64 `json:"percent"`
    DurationSec int     `json:"duration_sec"`
    TS          int64   `json:"ts"`
}

func SaveTrade(t Trade) {
    b, _ := json.Marshal(t)
    rdb.LPush(ctx, "trades:list", b)
}
```

**Satu posisi tertutup → satu `SaveTrade`.** Cegah duplikasi dengan idempotensi di lapisan atas (mis. kunci `closed:{mint}:{ts}`) jika pipeline bisa memanggil dua kali — sesuai DoD “tidak duplicate log”.

---

## 5. Integrasi ke alur SELL (wajib)

Di **monitor** (PR-002), **setelah** `SafeSellWithValidation` sukses (dan Anda punya estimasi / nilai final sell dalam SOL):

```go
sellSOL := lastQuote // atau hasil final sell estimate / amount dari RPC

pnlSOL := sellSOL - buySOL
percent := (pnlSOL / buySOL) * 100

trade := Trade{
    Mint:        mint,
    BuySOL:      buySOL,
    SellSOL:     sellSOL,
    PnLSOL:      pnlSOL,
    Percent:     percent,
    DurationSec: int(time.Since(start).Seconds()),
    TS:          time.Now().Unix(),
}

store.SaveTrade(trade)
report.MaybeSendSummary()
```

`lastQuote` harus konsisten dengan definisi PR-002 (quote simulate sell) atau diganti nilai aktual pasca-konfirmasi RPC jika tersedia — yang penting **satu sumber kebenaran** per trade.

---

## 6. Mesin agregasi

```go
type Stats struct {
    Total     int
    Win       int
    Loss      int
    TotalPnL  float64
    AvgPnL    float64
    Winrate   float64
}

func ComputeStats(trades []Trade) Stats {
    s := Stats{}

    for _, t := range trades {
        s.Total++
        s.TotalPnL += t.PnLSOL

        if t.PnLSOL > 0 {
            s.Win++
        } else {
            s.Loss++
        }
    }

    if s.Total > 0 {
        s.AvgPnL = s.TotalPnL / float64(s.Total)
        s.Winrate = float64(s.Win) / float64(s.Total) * 100
    }

    return s
}
```

Trade dengan `PnLSOL == 0` masuk ke cabang `Loss` pada cuplikan di atas; jika ingin “break-even” terpisah, sesuaikan logika (belum wajib di PR-003).

---

## 7. Loss streak (sinyal risiko)

```go
func LossStreak(trades []Trade) int {
    streak := 0

    for _, t := range trades {
        if t.PnLSOL < 0 {
            streak++
        } else {
            break
        }
    }

    return streak
}
```

**Urutan slice:** `LoadRecent` harus mengembalikan trade **terbaru dulu** (konsisten dengan `LPUSH` + `LRange` dari 0). Streak menghitung kerugian beruntun **dari trade paling baru ke belakang** sampai ada trade non-loss.

**Fungsi:** indikator kondisi market buruk atau potensi overtrade.

---

## 8. Memuat trade (terbatas)

```go
func LoadRecent(n int64) []Trade {
    vals, _ := rdb.LRange(ctx, "trades:list", 0, n-1).Result()

    trades := []Trade{}
    for _, v := range vals {
        var t Trade
        json.Unmarshal([]byte(v), &t)
        trades = append(trades, t)
    }

    return trades
}
```

**Aturan:** untuk agregasi dan Telegram, cukup **100–300** trade terakhir agar query tetap ringan (target DoD: di bawah 100 ms tergantung Redis dan ukuran payload).

---

## 9. Ringkasan Telegram

```go
func SendSummary(s Stats, streak int) {

    msg := fmt.Sprintf(`
RLANGGA REPORT

Trades: %d
Winrate: %.2f%%
Total PnL: %.4f SOL
Avg: %.4f SOL
Loss Streak: %d
`,
        s.Total,
        s.Winrate,
        s.TotalPnL,
        s.AvgPnL,
        streak,
    )

    sendTelegram(msg)
}
```

### Pemicu (`MaybeSendSummary`)

- Setiap **N** trade baru (mis. 5), **atau**
- Setiap **X** menit (timer / cron ringan di worker)

Implementasi menyimpan counter trade sejak laporan terakhir atau `last_report_ts` di Redis.

---

## 10. Alur (ringkas)

```text
SELL sukses (RPC/executor)
    → hitung buy/sell/pnl/durasi
    → store.SaveTrade (append-only)
    → guard.UpdateDailyLoss (PR-005, jika pnl < 0)
    → report.MaybeSendSummary (jika pemicu terpenuhi)
    → (opsional) aggregate untuk dashboard internal
```

---

## 11. Pengujian

| Jenis | Fokus |
|-------|--------|
| Unit | Agregasi, rata-rata, win rate, loss streak |
| Skenario | Semua menang; semua kalah; campuran; urutan streak |

---

## 12. Definition of done (final)

| Kriteria | Status |
|----------|--------|
| Semua trade tertutup tercatat | [ ] |
| Tidak duplicate log (idempotensi save) | [ ] |
| PnL akurat (SOL-based, definisi konsisten) | [ ] |
| Agregasi benar | [ ] |
| Win rate benar | [ ] |
| Loss streak sesuai urutan terbaru | [ ] |
| Laporan terkirim (Telegram) | [ ] |
| Query agregasi cepat (target di bawah 100 ms untuk N terbatas) | [ ] |

---

## Insight

**PR-003 = “mata” sistem:** tanpa data terstruktur, optimasi dan scale buta. Dengan ini ada **feedback loop** terukur.

**Kunci:** **no data = no optimization**

---

## Output setelah merge

| Tanpa PR-003 | Dengan PR-003 |
|--------------|----------------|
| Trading tanpa visibilitas | Performa riil terlihat |
| Tuning TP/SL menebak | Keputusan berdasarkan metrik |
| Scale tanpa dasar | Keputusan scale berdasarkan win rate / streak / total PnL |

---

## Rujukan

- Kontrak env (Telegram / laporan): [rlangga-env-contract.md](./rlangga-env-contract.md) §8  
- Hazard produksi (float loss, kuota): [rlangga-production-hazards-and-fixes.md](./rlangga-production-hazards-and-fixes.md)  
- PR-001: [rlangga-pr-001-core-engine-recovery-validation.md](./rlangga-pr-001-core-engine-recovery-validation.md)  
- PR-002: [rlangga-pr-002-adaptive-exit-pnl.md](./rlangga-pr-002-adaptive-exit-pnl.md)  
- PR-004 (multi bot, log per bot): [rlangga-pr-004-multi-bot.md](./rlangga-pr-004-multi-bot.md)  
- PR-005 (profit guard, `stats:daily_loss`): [rlangga-pr-005-profit-guard.md](./rlangga-pr-005-profit-guard.md)  
- Blueprint: [rlangga-blueprint-v2.md](./rlangga-blueprint-v2.md)  
- Stack: [rlangga-full-stack.md](./rlangga-full-stack.md)
