# PR-001 — core engine + recovery + validation

**ID:** PR-001  
**Jenjang dokumen:** spesifikasi deliverable — turunan dari [rlangga-blueprint-v2.md](./rlangga-blueprint-v2.md)  
**Tujuan:** mesin eksekusi inti yang *resilient* dan *tervalidasi* (bukan adaptive exit penuh; itu PR-002).

---

## 1. Ruang lingkup (final)

| Area | Termasuk |
|------|----------|
| Eksekusi | Buy / sell |
| Validasi RPC | Tx + wallet |
| Recovery | Startup + loop berkelanjutan |
| Redis | Lock mint + idempotency |
| Ketahanan | Retry, timeout (konfigurasi env), Docker `restart: always` |

---

## 2. Struktur proyek (minimum)

```text
cmd/worker/main.go

internal/
  app/
  executor/
  rpc/
  recovery/
  wallet/
  lock/
  idempotency/
  log/
```

Paket lain (monitor, exit, guard, dll.) boleh ada di repo lebar, tetapi **luar cakupan PR-001** kecuali dipanggil secara minimal.

---

## 3. Variabel lingkungan

| Kunci | Contoh / makna |
|-------|------------------|
| `REDIS_URL` | `redis:6379` |
| `PUMPPORTAL_URL` | URL primer eksekusi |
| `PUMPAPI_URL` | URL fallback |
| `RPC_URL` | Endpoint RPC (mis. Helius) |
| `TRADE_SIZE` | `0.1` (unit sesuai integrasi) |
| `TIMEOUT_MS` | `1500` |
| `RECOVERY_INTERVAL` | `10` (detik; loop recovery) |

---

## 4. Titik masuk (`main`)

```go
func main() {

    InitApp()

    // WAJIB: startup recovery
    recovery.RecoverAll()

    // WAJIB: continuous recovery
    go recovery.StartLoop()

    app.StartWorker()
}
```

`InitApp()` memuat konfigurasi, koneksi Redis, logger, dan dependensi RPC/executor.

---

## 5. Recovery (inti)

### Startup + isi `RecoverAll`

```go
func RecoverAll() {

    tokens := rpc.GetWalletTokens()

    for _, t := range tokens {
        if t.Amount > 0 {
            executor.SafeSellWithValidation(t.Mint)
        }
    }
}
```

### Loop berkelanjutan

```go
func StartLoop() {

    for {
        RecoverAll()
        time.Sleep(10 * time.Second)
    }
}
```

Interval sleep harus terhubung ke `RECOVERY_INTERVAL` (bukan magic number di produksi).

---

## 6. Modul RPC (wajib)

Polling konfirmasi transaksi sampai batas iterasi / timeout:

```go
func WaitTxConfirmed(sig string) bool {

    for i := 0; i < 10; i++ {

        status := getTxStatus(sig)

        if status == "confirmed" {
            return true
        }

        time.Sleep(300 * time.Millisecond)
    }

    return false
}
```

`getTxStatus` mengimplementasikan panggilan RPC sesuai `RPC_URL`. Tanpa konfirmasi yang konsisten, eksekusi tidak boleh dianggap sukses (**no RPC = no trust**).

---

## 7. Idempotency (Redis)

Mencegah pemrosesan ganda untuk event/mint yang sama dalam jendela singkat:

```go
func IsDuplicate(mint string) bool {
    ok, _ := rdb.SetNX(ctx, "event:"+mint, "1", 10*time.Second).Result()
    return !ok
}
```

`rdb` dan `ctx` berasal dari koneksi Redis aplikasi.

---

## 8. Lock mint (Redis)

Satu mint satu pemilik operasi:

```go
func LockMint(mint string) bool {
    ok, _ := rdb.SetNX(ctx, "mint:"+mint, "1", 5*time.Minute).Result()
    return ok
}

func UnlockMint(mint string) {
    rdb.Del(ctx, "mint:"+mint)
}
```

**Catatan:** Cuplikan di atas memakai pola `SetNX` — pastikan API `go-redis` sesuai versi (mis. `.Result()`). Lepas kunci setelah sell selesai atau saat path gagal (lihat alur worker).

---

## 9. Buy + validasi

Fallback API: PumpPortal → PumpAPI.

```go
func BuyAndValidate(mint string) bool {

    sig, err := pumpPortalBuy(mint)
    if err != nil {
        sig, err = pumpApiBuy(mint)
        if err != nil {
            return false
        }
    }

    return rpc.WaitTxConfirmed(sig)
}
```

---

## 10. Sell + validasi (retry eksplisit)

```go
func SafeSellWithValidation(mint string) {

    for i := 0; i < 5; i++ {

        sig, err := sell(mint)

        if err == nil && rpc.WaitTxConfirmed(sig) {
            return
        }

        time.Sleep(500 * time.Millisecond)
    }

    log.Error("SELL FAILED: " + mint)
}
```

Setelah retry habis, **wajib** log error (tidak boleh *silent fail*). Recovery loop berikutnya harus dapat mencoba lagi menjual sisa posisi.

---

## 11. Alur worker (`HandleMint`)

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

    // sementara (PR-002: adaptive exit)
    time.Sleep(10 * time.Second)

    executor.SafeSellWithValidation(mint)
}
```

**TODO PR-002:** ganti `time.Sleep` tetap dengan `monitor` + `exit` adaptif.

**Pembaruan:** di kode saat ini, alur setelah BUY sudah memakai **monitor + exit adaptif** (bukan `Sleep` tetap). Cuplikan di atas tetap ada sebagai **baseline historis PR-001**. Detail selisih dokumen vs repo: [implementation-vs-spec.md](./implementation-vs-spec.md).

**Alur worker kanonik (setelah PR-002–PR-005):** guard → *(opsional)* filter / suffix `pump` → idempotency → lock → orchestrator/bot → buy → monitor adaptif / multi-bot — lihat [PR-004 §5](./rlangga-pr-004-multi-bot.md), [PR-005](./rlangga-pr-005-profit-guard.md), [implementation-vs-spec.md](./implementation-vs-spec.md) §9. Cuplikan di atas hanya baseline PR-001.

Pastikan `UnlockMint` dipanggil pada path sukses setelah sell (atau kebijakan lock yang konsisten) agar tidak mengunci mint selamanya.

---

## 12. Docker

```yaml
version: "3"

services:
  redis:
    image: redis:7
    restart: always

  worker:
    build: .
    restart: always
    depends_on:
      - redis
```

Lingkungan worker memuat `REDIS_URL` dan variabel di bagian 3.

---

## 13. Skenario uji (wajib)

| Skenario | Ekspektasi |
|----------|------------|
| Bot / worker restart | Recovery startup + loop jalan |
| Wallet berisi token | Auto sell sampai bersih (sesuai RPC) |
| Buy gagal | Tidak lanjut ke hold panjang; lock dilepas jika perlu |
| Sell gagal | Retry; tidak diam-diam sukses |
| Event duplikat | Di-skip (idempotency) |

---

## 14. Definition of done (final)

| Kriteria | Status |
|----------|--------|
| Recovery startup jalan | [ ] |
| Recovery loop jalan | [ ] |
| Validasi RPC aktif untuk tx kritis | [ ] |
| Tidak ada posisi nyangkut (invariant + recovery) | [ ] |
| Tidak double trade untuk event sama | [ ] |
| Sell tidak silent fail | [ ] |
| Sistem survive restart (Docker + recovery) | [ ] |

---

## 15. Cakupan kegagalan

| Situasi | Respons |
|---------|---------|
| Server / proses mati | Recovery loop setelah hidup lagi |
| Tx gagal | Blok / tidak anggap sukses (`WaitTxConfirmed`) |
| Sell gagal | Retry + log; recovery berikutnya |
| State mismatch | Recovery scan memperbaiki dengan menjual/token cleanup |

---

## Kunci penguncian

**PR-001 = core execution system (resilient + validated).**

---

## Output setelah merge

| Target | Keterangan |
|--------|------------|
| Operasi 24/7 | Worker + Redis + restart policy |
| Tidak takut restart | Startup recovery + loop |
| Tidak *blind state* | RPC untuk tx + wallet |
| Tidak silent error | Log pada kegagalan sell; validasi tx |

---

## Rujukan

- Kontrak env: [rlangga-env-contract.md](./rlangga-env-contract.md)  
- Hazard produksi (race, recovery, lock, RPC): [rlangga-production-hazards-and-fixes.md](./rlangga-production-hazards-and-fixes.md)  
- Blueprint governance & guard: [rlangga-blueprint-v2.md](./rlangga-blueprint-v2.md)  
- Layout repo & modul: [rlangga-repo-structure.md](./rlangga-repo-structure.md)  
- Stack keseluruhan: [rlangga-full-stack.md](./rlangga-full-stack.md)  
- Lanjutan adaptive exit + PnL: [rlangga-pr-002-adaptive-exit-pnl.md](./rlangga-pr-002-adaptive-exit-pnl.md)  
- Trade store + metrik + laporan: [rlangga-pr-003-pnl-validation-analytics.md](./rlangga-pr-003-pnl-validation-analytics.md)  
- Multi bot + scale: [rlangga-pr-004-multi-bot.md](./rlangga-pr-004-multi-bot.md)  
- Profit guard (daily loss, kill switch): [rlangga-pr-005-profit-guard.md](./rlangga-pr-005-profit-guard.md)
