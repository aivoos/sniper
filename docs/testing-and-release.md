# Pengujian yang benar → siap live

Dokumen ini menjelaskan **urutan tes yang masuk akal**, memisahkan mitos dari praktik, dan mengarah ke [go-live-checklist.md](./go-live-checklist.md).  
Detail standar unit/simulasi/integrasi tetap di **[rlangga-test-standard.md](./rlangga-test-standard.md)**.

---

## Yang salah (mitos / anti-pola)

| Salah | Kenapa |
|--------|--------|
| Menganggap **satu angka coverage tinggi** = aman mainnet | Coverage hanya mengukur baris kode yang dieksekusi oleh tes **unit**; tidak menggantikan uji integrasi, RPC nyata, atau perilaku di beban. |
| **`make test` lokal beda dengan CI** (ambang / flag berbeda) | Gate “hijau” harus **sama** antara lokal dan GitHub Actions, supaya merge tidak kejutan. |
| Hanya `go test` tanpa **staging** | Konfigurasi `.env`, Redis, Pump API, dan stream WS punya interaksi yang tidak tertangkap unit test murni. |
| Langsung nominal besar setelah tes lulus | Risiko slippage, rate limit, dan bug operasional — naikkan ukuran secara bertahap. |

---

## Urutan yang benar (dari kode → produksi)

### 1. Gate kualitas kode (setiap commit / PR)

1. **`go fmt` / `go vet`** — gaya dan analisis statis.
2. **`go test ./internal/...`** — tes unit paket internal (tanpa `-race` jika belum ada gcc: tetap valid).
3. **Sama seperti CI:** `make ci` (lihat Makefile) — fmt, vet, tes + ambang coverage yang **sama dengan** `.github/workflows/ci.yml`.

Lokal tanpa gcc: `make test` (tanpa race). Untuk parity penuh dengan CI (race): pasang `build-essential` lalu jalankan perintah di workflow atau target `make test-race` jika tersedia.

### 2. Uji integrasi manual / staging (wajib sebelum uang nyata)

- Worker dengan **Redis** nyata, `.env` staging.
- **`SIMULATE_ENGINE=1`** atau **`SIMULATE_TRADING=1`** untuk observasi alur tanpa (atau dengan minimal) eksekusi on-chain.
- **`RPC_STUB=0`** + **`RPC_URL` / `RPC_URLS`** ke endpoint **mainnet** (atau RPC staging Anda) — verifikasi `WaitTxConfirmed` dan failover RPC.
- Restart worker, cek log, cek Redis (trades, guard).

### 3. Nominal kecil di jaringan target

- Matikan mode simulasi (`SIMULATE_*=0`), ukuran **`TRADE_SIZE`** minimal, **`MAX_DAILY_TRADES`** terbatas.
- Pantau beberapa jam / hari: kill switch, laporan, recovery.

### 4. Go-live

- Ikuti **[go-live-checklist.md](./go-live-checklist.md)** (wallet RPC, token recovery, pump gateway, supervisor, alert).

---

## Coverage: ambang di repo

- **Total coverage gabungan** (`go tool cover -func`) adalah **metrik agregat**; nilai ~77% tidak berarti 23% kode “tidak penting” — bisa jadi path RPC/pump yang sulit di-mock.
- Ambang minimum diset **konsisten** antara `Makefile` dan `.github/workflows/ci.yml` (variabel `COVERAGE_MIN`), dengan margin di bawah angka aktual agar CI tidak gagal karena fluktuasi kecil.
- Menaikkan ambang secara bertahap + menambah tes **lebih berharga** daripada memburu angka tanpa skenario.

---

## Ringkas satu kalimat

**Benar:** fmt → vet → tes (parity CI) → staging dengan Redis + env nyata + simulasi → nominal kecil → checklist go-live.  
**Salah:** menganggap tes unit + angka coverage tinggi saja sudah cukup untuk live 24/7 dengan uang penuh.
