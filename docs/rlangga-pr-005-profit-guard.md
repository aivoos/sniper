# PR-005 — profit guard

**ID:** PR-005  
**Jenjang dokumen:** spesifikasi deliverable — turunan dari [rlangga-blueprint-v2.md](./rlangga-blueprint-v2.md)  
**Prasyarat:** [PR-003](./rlangga-pr-003-pnl-validation-analytics.md) (`SaveTrade`, kunci `stats:daily_loss`), [PR-001](./rlangga-pr-001-core-engine-recovery-validation.md) (recovery / SELL selalu diizinkan)

**Objektif**

| Target | Keterangan |
|--------|------------|
| Stop loss global harian | Akumulasi kerugian SOL per hari |
| Perlindungan saldo | Minimum balance sebelum buka posisi baru |
| Pembatas entri | Gate BUY; **kuota harian** (`MAX_DAILY_TRADES`, blueprint §7) belum dijabarkan sebagai env terpisah di PR-005 — tambahkan counter Redis + cek di `CanTrade` agar selaras blueprint |
| *Safe stop* | Tidak ada BUY baru saat guard aktif; **SELL tetap diizinkan** |
| Alerting | Telegram saat kill switch / kondisi kritis |
| Reset | Mekanisme reset harian / manual |

---

## 1. Ruang lingkup (final)

| Area | Termasuk |
|------|----------|
| Pelacakan loss harian | Redis `stats:daily_loss` |
| Kill switch | Bandingkan akumulasi vs `MAX_DAILY_LOSS` |
| Balance guard | `MIN_BALANCE` |
| Trade gate | `CanTrade` — filter **sebelum** BUY |
| Alert | Telegram |
| Reset | Nol-kan counter (cron / manual) |

---

## 2. Pembaruan modul

```text
internal/guard/
```

Logika guard dipanggil dari **worker** (`HandleMint`) sebelum idempotency/lock BUY. **Recovery** dan **SELL paksa** tidak boleh diblokir oleh gate ini (lihat perilaku aman).

---

## 3. Konfigurasi

| Variabel | Contoh | Makna |
|----------|--------|--------|
| `MAX_DAILY_LOSS` | `0.5` | Batas akumulasi rugi harian (SOL) — trigger kill switch |
| `MIN_BALANCE` | `0.2` | Saldo minimum untuk mengizinkan BUY |
| `ENABLE_TRADING` | `true` | Matikan semua BUY baru jika `false` (operasi / maintenance) |

---

## 4. Pelacakan loss harian (Redis)

**Kunci:** `stats:daily_loss` (konsisten dengan [PR-003](./rlangga-pr-003-pnl-validation-analytics.md)).

Pembaruan saat trade tertutup rugi — **setelah** `SaveTrade` berhasil, dari nilai PnL trade:

```go
func UpdateDailyLoss(pnl float64) {

    if pnl < 0 {
        rdb.IncrByFloat(ctx, "stats:daily_loss", -pnl)
    }
}
```

**Integrasi:** panggil `guard.UpdateDailyLoss(trade.PnLSOL)` di pipeline PR-003 tepat setelah `SaveTrade` (atau di dalam wrapper `SaveTrade` jika satu transaksi logika).

---

## 5. Kill switch

```go
func IsKillSwitchTriggered() bool {

    loss, _ := rdb.Get(ctx, "stats:daily_loss").Float64()

    return loss >= cfg.MaxDailyLoss
}
```

**Catatan implementasi:** pada `go-redis`, ambil string lalu `strconv.ParseFloat` atau gunakan `Float64()` jika tersedia di versi yang dipakai — sesuaikan dengan API aktual.

---

## 6. Balance guard

```go
func HasEnoughBalance(balance float64) bool {
    return balance >= cfg.MinBalance
}
```

`balance` berasal dari RPC / wallet (SOL).

---

## 7. Trade gate (kontrol entri)

```go
func CanTrade(balance float64) bool {

    if !cfg.EnableTrading {
        return false
    }

    if IsKillSwitchTriggered() {
        return false
    }

    if !HasEnoughBalance(balance) {
        return false
    }

    return true
}
```

**Hanya memutus BUY baru.** Path **recovery** (`RecoverAll`, `SafeSellWithValidation`) **tidak** memanggil `CanTrade` untuk membuka posisi — hanya untuk likuidasi.

---

## 8. Integrasi worker

Cuplikan di bawah menyoroti **gate BUY** (`CanTrade`). Urutan penuh `HandleMint` termasuk filter opsional **sebelum** idempotency — sama dengan [PR-004 §5](./rlangga-pr-004-multi-bot.md) dan `internal/app/app.go`.

```go
func HandleMint(mint string) {

    balance := wallet.GetSOLBalance()

    if !guard.CanTrade(balance) {
        log.Info("TRADE BLOCKED")
        return
    }

    // Opsional: FILTER_REQUIRE_INITIAL_BUY, lalu FILTER_ANTI_RUG → filter.AllowMint
    // (lihat PR-004 §5 dan rlangga-env-contract.md §7c.1)

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

Urutan: **guard** → *(opsional)* filter / suffix → **idempotency** → **lock** → BUY → monitor — konsisten dengan [PR-004](./rlangga-pr-004-multi-bot.md).

---

## 9. Perilaku *safe stop*

| Aturan | Keterangan |
|--------|------------|
| Stop BUY | `CanTrade` mengembalikan `false` |
| Tetap izinkan SELL | Recovery + monitor exit memanggil `SafeSellWithValidation` tanpa cek gate BUY |
| Sistem tetap jalan | Loop recovery, RPC, logging tetap aktif |

**Invariant:** tidak meninggalkan posisi sengaja terkunci hanya karena guard — penjualan dan recovery tetap jalan ([PR-001](./rlangga-pr-001-core-engine-recovery-validation.md)).

---

## 10. Alert

```go
func SendKillSwitchAlert(loss float64) {

    msg := fmt.Sprintf(
        "RLANGGA KILL SWITCH\nLoss: %.4f SOL\nTrading stopped",
        loss,
    )

    sendTelegram(msg)
}
```

Panggil saat `IsKillSwitchTriggered()` baru pertama kali menjadi benar (edge detection) agar tidak spam; atau throttle per jam.

---

## 11. Reset harian

```go
func ResetDailyLoss() {
    rdb.Set(ctx, "stats:daily_loss", 0, 24*time.Hour)
}
```

**Pemicu:** cron (mis. tengah malam UTC atau WIB), atau perintah admin manual.

**Catatan:** TTL 24 jam pada kunci **bukan** pengganti jadwal kalender yang sempurna; alternatif: `SET stats:daily_loss 0` tanpa TTL + cron harian, atau kunci per hari `stats:daily_loss:YYYY-MM-DD`. Pilih satu strategi agar reset konsisten dengan zona waktu operasi.

---

## 12. Kegagalan & mitigasi

| Kasus | Respons |
|-------|---------|
| Redis gagal saat reset | Fallback aman: log error; jangan crash worker; pertimbangkan *circuit breaker* baca/tulis |
| *False trigger* | Override manual: `ENABLE_TRADING`, atau `SET` loss ke 0 setelah verifikasi, + alert |

---

## 13. Pengujian

| Jenis | Fokus |
|-------|--------|
| Unit | Kill switch threshold, balance guard, `CanTrade` |
| Skenario | Loss kecil → BUY masih boleh; loss besar → BUY diblok; saldo kecil → skip BUY |

---

## 14. Definition of done (final)

| Kriteria | Status |
|----------|--------|
| Kill switch aktif sesuai `MAX_DAILY_LOSS` | [ ] |
| Tidak ada BUY baru setelah trigger | [ ] |
| SELL / recovery tetap jalan | [ ] |
| Balance guard bekerja | [ ] |
| Alert terkirim (Telegram) | [ ] |
| Proses tidak crash saat Redis edge case | [ ] |
| Reset manual / terjadwal memungkinkan recovery operasi | [ ] |

---

## Insight

**PR-005 bukan untuk memaksimalkan profit**, melainkan **supaya sistem bertahan hidup** — batasi kerugian harian dan jaga saldo sebelum menambah risiko.

---

## Output setelah merge

| Tanpa PR-005 | Dengan PR-005 |
|--------------|----------------|
| Eksposur tak terbatas pada hari buruk | Kerugian harian terbungkus |
| BUY saat saldo kritis | Gate mencegah entri bodoh |
| Tanpa sinyal operasi | Alert + kill switch terukur |

---

## Rujukan

- Kontrak env (satu tabel): [rlangga-env-contract.md](./rlangga-env-contract.md)  
- Hazard produksi (recovery bypass guard, kuota, midnight window): [rlangga-production-hazards-and-fixes.md](./rlangga-production-hazards-and-fixes.md)  
- PR-001 (recovery, SELL prioritas): [rlangga-pr-001-core-engine-recovery-validation.md](./rlangga-pr-001-core-engine-recovery-validation.md)  
- PR-003 (`stats:daily_loss`, `SaveTrade`): [rlangga-pr-003-pnl-validation-analytics.md](./rlangga-pr-003-pnl-validation-analytics.md)  
- PR-004 (`HandleMint`): [rlangga-pr-004-multi-bot.md](./rlangga-pr-004-multi-bot.md)  
- Blueprint (dual guard, kuota, jendela waktu): [rlangga-blueprint-v2.md](./rlangga-blueprint-v2.md)  
- Stack: [rlangga-full-stack.md](./rlangga-full-stack.md)
