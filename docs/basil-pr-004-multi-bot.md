# PR-004 — multi bot

**ID:** PR-004  
**Jenjang dokumen:** spesifikasi deliverable — turunan dari [basil-blueprint-v2.md](./basil-blueprint-v2.md)  
**Prasyarat:** [PR-001](./basil-pr-001-core-engine-recovery-validation.md) (lock Redis global), [PR-002](./basil-pr-002-adaptive-exit-pnl.md) (monitor + adaptive exit)

**Objektif**

| Target | Keterangan |
|--------|------------|
| Beberapa worker paralel | Throughput lebih tinggi |
| Konfigurasi per bot | Hold / TP / SL / momentum berbeda per profil |
| Satu mint satu posisi | Lock Redis bersama → anti tabrakan |
| Deterministik | Aturan tetap untuk input + state yang sama |
| Scale via Docker | Replika worker, Redis tetap satu cluster |

---

## 1. Ruang lingkup (final)

| Area | Termasuk |
|------|----------|
| Konfigurasi multi bot | Profil `BotConfig` |
| Orchestrator | Penugasan round-robin |
| Lock | Redis **shared** antar proses / replika |
| Exit | `ShouldSellAdaptiveBot` memakai field per bot |
| Log | Prefix / konteks nama bot |
| Deploy | Scale worker (Compose / Swarm / replika manual) |

---

## 2. Pembaruan modul

```text
internal/
  bot/
  orchestrator/
```

Paket lain (executor, lock, monitor, dll.) diperluas; inti PR-004 adalah **profil bot + assignment + exit per bot**.

---

## 3. Konfigurasi bot (inti)

```go
type BotConfig struct {
    Name         string
    MinHold      int
    MaxHold      int
    TakeProfit   float64
    StopLoss     float64
    PanicLoss    float64
    MomentumDrop float64
}
```

### Bot default (contoh)

```go
var bots = []BotConfig{
    {
        Name:         "bot-10s",
        MinHold:      5,
        MaxHold:      10,
        TakeProfit:   7,
        StopLoss:     5,
        PanicLoss:    8,
        MomentumDrop: 2.5,
    },
    {
        Name:         "bot-15s",
        MinHold:      5,
        MaxHold:      15,
        TakeProfit:   8,
        StopLoss:     5,
        PanicLoss:    8,
        MomentumDrop: 3,
    },
}
```

Konfigurasi dapat dipindah ke file / env pada produksi; cuplikan di atas memperlihatkan kontrak struct.

---

## 4. Orchestrator (round-robin)

```go
var idx = 0

func NextBot() BotConfig {
    b := bots[idx]
    idx = (idx + 1) % len(bots)
    return b
}
```

**Fungsi:** membagi beban pilihan profil antar event masuk (bukan menggandakan trade pada mint yang sama — itu dicegah lock).

**Produksi:** bungkus kenaikan `idx` dengan mutex atau atomik jika banyak goroutine memanggil `NextBot` bersamaan.

---

## 5. Alur worker (diperbarui)

**PR-005:** gate `guard.CanTrade` di awal (setelah baca saldo). Detail perilaku guard: [basil-pr-005-profit-guard.md](./basil-pr-005-profit-guard.md).

```go
func HandleMint(mint string) {

    balance := wallet.GetSOLBalance()

    if !guard.CanTrade(balance) {
        log.Info("TRADE BLOCKED")
        return
    }

    if idempotency.IsDuplicate(mint) {
        return
    }

    if !lock.LockMint(mint) {
        return
    }

    bot := orchestrator.NextBot()

    success := executor.BuyAndValidate(mint)
    if !success {
        lock.UnlockMint(mint)
        return
    }

    MonitorPositionWithBot(mint, cfg.TradeSize, bot)
}
```

**Unlock:** sama seperti PR-001 / PR-002 — pastikan `UnlockMint` pada semua path keluar (gagal beli, setelah sell, dll.).

---

## 6. Monitor (per bot)

```go
func MonitorPositionWithBot(mint string, buySOL float64, bot BotConfig) {

    state := &PositionState{
        BuySOL: buySOL,
    }

    start := time.Now()

    for {

        elapsed := int(time.Since(start).Seconds())

        quote := GetSellQuote(mint)
        pnl := CalcPnL(buySOL, quote)

        if ShouldSellAdaptiveBot(pnl, elapsed, state, bot) {

            executor.SafeSellWithValidation(mint)

            return
        }

        time.Sleep(500 * time.Millisecond)
    }
}
```

Interval tidur sebaiknya mengikuti `QUOTE_INTERVAL_MS` (PR-002), bukan angka tetap.

---

## 7. Mesin exit (per `BotConfig`)

```go
func ShouldSellAdaptiveBot(
    pnl float64,
    elapsed int,
    state *PositionState,
    bot BotConfig,
) bool {

    if pnl > state.PeakPnL {
        state.PeakPnL = pnl
    }

    if pnl <= -bot.PanicLoss {
        return true
    }

    if elapsed < 2 {
        if pnl >= bot.TakeProfit {
            return true
        }
        return false
    }

    if pnl <= -bot.StopLoss {
        return true
    }

    if pnl >= bot.TakeProfit && elapsed >= bot.MinHold {
        return true
    }

    drop := state.PeakPnL - pnl
    if drop >= bot.MomentumDrop {
        return true
    }

    if elapsed >= bot.MaxHold {
        return true
    }

    return false
}
```

**Catatan:** jendela *grace* saat ini memakai konstanta `2` detik; opsional selaras dengan PR-002 dengan menambah `GraceSeconds` ke `BotConfig` agar semua profil bisa menyesuaikan noise window.

---

## 8. Anti collision (kritis)

| Aturan | Implementasi |
|--------|----------------|
| Lock global | Kunci Redis `mint:{mint}` (atau kontrak PR-001) |
| Semua bot / worker share lock | Endpoint Redis sama untuk semua replika |
| Satu mint satu trade aktif | `SetNX` gagal → worker lain tidak membuka posisi sama |

Tanpa ini, multi bot bisa **saling melawan** pada mint yang sama.

---

## 9. Log per bot

```go
log.Info(fmt.Sprintf("[%s] BUY %s", bot.Name, mint))
log.Info(fmt.Sprintf("[%s] SELL %s PnL %.2f", bot.Name, mint, pnl))
```

Untuk [PR-003](./basil-pr-003-pnl-validation-analytics.md), pertimbangkan field `bot_name` pada `Trade` jika metrik per profil diperlukan.

---

## 10. Docker scale

### Opsi A — replika (Compose Swarm / stack)

```yaml
worker:
  build: .
  restart: always
  deploy:
    replicas: 2
```

`deploy.replicas` berlaku di **Docker Swarm** atau file Compose yang di-deploy ke orchestrator yang mendukung. Untuk `docker compose` biasa di satu host, gunakan opsi B atau definisi beberapa service `worker1` / `worker2`.

### Opsi B — manual

```bash
docker run … worker
docker run … worker
```

Semua instans memakai **`REDIS_URL` yang sama** → lock dan idempotency tetap konsisten.

---

## 11. Pengujian (skenario)

| Skenario | Ekspektasi |
|----------|------------|
| Dua worker paralel | Tidak ada dua posisi pada mint yang sama |
| Dua profil bot | Parameter hold/TP/SL terlihat berbeda di perilaku exit |
| Race | Lock mencegah double trade pada mint sama |

---

## 12. Definition of done (final)

| Kriteria | Status |
|----------|--------|
| Multi bot / multi worker berjalan | [ ] |
| Tidak double trade (mint sama) | [ ] |
| Konfigurasi per bot dipakai di exit | [ ] |
| Tidak race pada mint (lock) | [ ] |
| Log memuat identitas bot | [ ] |
| Scaling worker teruji (Redis shared) | [ ] |

---

## Insight

Multi bot **tidak** otomatis membuat strategi “lebih pintar”; fungsinya menambah **jumlah peluang eksekusi** dengan aman, asalkan invariant lock dijaga.

**Kunci:** **multi bot = scale execution**, bukan mengganti definisi strategi tunggal secara ajaib.

---

## Strategi scale (operasi)

| Tahap | Skala | Tujuan |
|-------|--------|--------|
| 1 | 1 bot | Validasi |
| 2 | 2 bot | Baseline |
| 3 | 3–4 bot | Optimal awal throughput |

---

## Output setelah merge

| Aspek | Keterangan |
|--------|------------|
| Volume | Lebih banyak trade yang bisa dilayani (parallel) |
| Peluang | Lebih banyak event yang masuk pipeline (dengan guard global tetap) |
| Keamanan | Tetap aman jika Redis lock + idempotency dipakai bersama |

---

## Rujukan

- Kontrak env: [basil-env-contract.md](./basil-env-contract.md)  
- Hazard produksi (orchestrator atomik, lock TTL, RPC recovery): [basil-production-hazards-and-fixes.md](./basil-production-hazards-and-fixes.md)  
- PR-001 (lock): [basil-pr-001-core-engine-recovery-validation.md](./basil-pr-001-core-engine-recovery-validation.md)  
- PR-002 (monitor / exit): [basil-pr-002-adaptive-exit-pnl.md](./basil-pr-002-adaptive-exit-pnl.md)  
- PR-003 (metrik / trade log): [basil-pr-003-pnl-validation-analytics.md](./basil-pr-003-pnl-validation-analytics.md)  
- Stack multi-bot: [basil-full-stack.md](./basil-full-stack.md)  
- Blueprint: [basil-blueprint-v2.md](./basil-blueprint-v2.md)
