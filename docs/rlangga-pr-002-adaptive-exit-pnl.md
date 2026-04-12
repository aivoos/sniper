# PR-002 — adaptive exit + PnL

**ID:** PR-002  
**Jenjang dokumen:** spesifikasi deliverable — turunan dari [rlangga-blueprint-v2.md](./rlangga-blueprint-v2.md)  
**Prasyarat:** [PR-001](./rlangga-pr-001-core-engine-recovery-validation.md) (eksekusi, recovery, validasi RPC, lock, idempotency)

**Objektif**

| Target | Keterangan |
|--------|------------|
| Exit tidak fixed | Keputusan dari PnL + waktu + momentum |
| PnL riil | Berbasis *quote* simulate sell (SOL), bukan asumsi harga token mentah |
| Sadar momentum | *Peak* tracking + *momentum drop* |
| Deterministik | Aturan sama untuk input sama; tetap terintegrasi RPC untuk tx |
| Terintegrasi PR-001 | `SafeSellWithValidation`, lock, idempotency tidak di-bypass |

---

## 1. Ruang lingkup (final)

| Area | Termasuk |
|------|----------|
| Quote | Simulate sell (PumpPortal → fallback PumpAPI) |
| PnL | Perhitungan persentase SOL-based |
| Exit | Mesin adaptif (`ShouldSellAdaptive`) |
| State | `PositionState` + *peak* PnL |
| Monitor | Loop pemantauan posisi sampai exit |
| Worker | `HandleMint` memanggil `MonitorPosition` menggantikan sleep tetap PR-001 |

---

## 2. Modul (baru / fokus PR-002)

```text
internal/
  quote/
  pnl/
  exit/
  monitor/
```

Modul PR-001 (`executor`, `rpc`, `recovery`, `lock`, `idempotency`, dll.) tetap dipakai; PR-002 **menambahkan** perilaku di atas dan mengubah alur pasca-buy.

---

## 3. Konfigurasi

| Variabel lingkungan | Contoh | Makna |
|---------------------|--------|--------|
| `GRACE_SECONDS` | `2` | Jendela awal: kurangi exit noise; TP besar tetap boleh (lihat mesin exit) |
| `MIN_HOLD` | `5` | Detik minimum sebelum TP “penuh” (selain aturan grace) |
| `MAX_HOLD` | `15` | Batas hold absolut |
| `TP_PERCENT` | `7` | Ambang take profit (%) |
| `SL_PERCENT` | `5` | Ambang stop loss (%) |
| `PANIC_SL` | `8` | Ambang panic cut (%) |
| `MOMENTUM_DROP` | `2.5` | Penurunan dari *peak* PnL → jual |
| `QUOTE_INTERVAL_MS` | `500` | Interval poll quote di loop monitor |

Loader konfigurasi memetakan ke struct `Config` (mis. `TakeProfit` ← `TP_PERCENT`, `StopLoss` ← `SL_PERCENT`, `PanicSL` ← `PANIC_SL`, `MomentumDrop` ← `MOMENTUM_DROP`, `GraceSeconds` ← `GRACE_SECONDS`, `MinHold` ← `MIN_HOLD`, `MaxHold` ← `MAX_HOLD`).

---

## 4. Model PnL (terkunci)

Persentase perubahan terhadap biaya masuk dalam SOL (basis *simulate sell*):

```go
func CalcPnL(buySOL, quoteSOL float64) float64 {
    return (quoteSOL - buySOL) / buySOL * 100
}
```

**Basis:** nilai yang dipakai untuk `quoteSOL` berasal dari **simulate sell** (keluaran quote API), bukan sekadar harga token statis.

---

## 5. Quote (simulate sell)

Fallback sama seperti eksekusi: portal primer, API cadangan.

```go
func GetSellQuote(mint string) float64 {

    sol, err := pumpPortalQuote(mint)
    if err == nil {
        return sol
    }

    sol, _ = pumpApiQuote(mint)
    return sol
}
```

---

## 6. State tracking

```go
type PositionState struct {
    BuySOL  float64
    PeakPnL float64
}
```

`PeakPnL` diperbarui saat PnL naik; dipakai untuk deteksi *momentum drop*.

---

## 7. Adaptive exit engine (inti)

```go
func ShouldSellAdaptive(
    pnl float64,
    elapsed int,
    state *PositionState,
    cfg Config,
) bool {

    // update peak
    if pnl > state.PeakPnL {
        state.PeakPnL = pnl
    }

    // panic loss
    if pnl <= -cfg.PanicSL {
        return true
    }

    // grace: noise window
    if elapsed < cfg.GraceSeconds {
        if pnl >= cfg.TakeProfit {
            return true
        }
        return false
    }

    // SL
    if pnl <= -cfg.StopLoss {
        return true
    }

    // TP (minimal hold)
    if pnl >= cfg.TakeProfit && elapsed >= cfg.MinHold {
        return true
    }

    // momentum drop
    drop := state.PeakPnL - pnl
    if drop >= cfg.MomentumDrop {
        return true
    }

    // max hold
    if elapsed >= cfg.MaxHold {
        return true
    }

    return false
}
```

**Kunci desain:** `exit = pnl + momentum + time` (panic, grace, SL/TP, *drop* dari peak, *max hold*).

---

## 8. Monitor loop

```go
func MonitorPosition(mint string, buySOL float64) {

    state := &PositionState{
        BuySOL:  buySOL,
        PeakPnL: 0,
    }

    start := time.Now()

    for {

        elapsed := int(time.Since(start).Seconds())

        quote := GetSellQuote(mint)
        pnl := CalcPnL(buySOL, quote)

        if ShouldSellAdaptive(pnl, elapsed, state, cfg) {

            executor.SafeSellWithValidation(mint)

            return
        }

        time.Sleep(500 * time.Millisecond)
    }
}
```

**Produksi:** ganti `500 * time.Millisecond` dengan `time.Duration(cfg.QuoteIntervalMs) * time.Millisecond` agar selaras dengan `QUOTE_INTERVAL_MS`.

---

## 9. Pembaruan worker (integrasi PR-001)

```go
func HandleMint(mint string) {

    if idempotency.IsDuplicate(mint) {
        return
    }

    if !lock.LockMint(mint) {
        return
    }

    success := executor.BuyAndValidate(mint)
    if !success {
        lock.UnlockMint(mint)
        return
    }

    buySOL := cfg.TradeSize

    MonitorPosition(mint, buySOL)
}
```

**Lock:** setelah `MonitorPosition` selesai (sell atau return), pastikan **`UnlockMint(mint)`** dipanggil pada semua path agar konsisten dengan PR-001 (dokumen PR-001 menjelaskan kebijakan unlock; implementasi wajib menutup semua cabang).

**Integrasi lanjutan:** cuplikan di atas adalah cakupan PR-002 saja. Untuk **guard** (`CanTrade`), **multi-bot** (`NextBot`, `MonitorPositionWithBot`), dan **trade log** (`SaveTrade`), gabungkan dengan [PR-004](./rlangga-pr-004-multi-bot.md), [PR-005](./rlangga-pr-005-profit-guard.md), [PR-003](./rlangga-pr-003-pnl-validation-analytics.md).

---

## 10. Perilaku sistem (intuisi)

| Kondisi | Perilaku |
|---------|----------|
| Token kuat | Hold lebih lama, peak naik, exit saat momentum drop atau TP/max hold |
| Token lemah | Exit lebih cepat lewat SL, panic, atau drop dari peak kecil |
| Noise awal | *Grace* membatasi exit kecuali TP ekstrem |

---

## 11. Manfaat (edge)

| Manfaat | Keterangan |
|---------|------------|
| Tidak over-hold | `MaxHold` + momentum drop |
| Kurangi fake pump | Keluar saat retracement dari peak (bukan hanya timer) |
| Lock profit saat momentum turun | *Momentum drop* |
| Cut loss cepat | Panic + SL |

---

## 12. Pengujian

| Jenis | Fokus |
|-------|--------|
| Unit | `CalcPnL`, logika *momentum drop*, pemicu TP/SL/panic |
| Simulasi | Pump cepat → hold → exit dekat peak; pump lemah → exit cepat; flat → exit waktu |

Standar wajib (piramida, CI, determinisme): [rlangga-test-standard.md](./rlangga-test-standard.md).

---

## 13. Definition of done (final)

| Kriteria | Status |
|----------|--------|
| PnL akurat (quote-based) | [x] |
| Adaptive exit aktif | [x] |
| Momentum drop bekerja | [x] |
| Tidak exit terlalu agresif di noise (grace) | [x] |
| Tidak over-hold (max hold + drop) | [x] |
| Integrasi PR-001 stabil (sell + RPC + lock) | [x] |

---

## Kunci penguncian

**exit = pnl + momentum + time**

---

## Output setelah merge

| Sebelum (PR-001 saja) | Sesudah PR-002 |
|------------------------|----------------|
| Exit berbasis waktu tetap | **Context-aware:** quote + PnL + peak + aturan waktu |
| Bot “time-based bodoh” | **Context-aware execution system** (tetap deterministik dalam definisi input) |

---

## Rujukan

- Standar tes: [rlangga-test-standard.md](./rlangga-test-standard.md)  
- Kontrak env: [rlangga-env-contract.md](./rlangga-env-contract.md)  
- Hazard produksi (quote stale, double sell, dust): [rlangga-production-hazards-and-fixes.md](./rlangga-production-hazards-and-fixes.md)  
- PR-001: [rlangga-pr-001-core-engine-recovery-validation.md](./rlangga-pr-001-core-engine-recovery-validation.md)  
- PR-003 (trade log + metrik + Telegram): [rlangga-pr-003-pnl-validation-analytics.md](./rlangga-pr-003-pnl-validation-analytics.md)  
- PR-004 (multi bot + scale): [rlangga-pr-004-multi-bot.md](./rlangga-pr-004-multi-bot.md)  
- Blueprint: [rlangga-blueprint-v2.md](./rlangga-blueprint-v2.md)  
- Stack: [rlangga-full-stack.md](./rlangga-full-stack.md)
