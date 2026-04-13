# Checklist go-live (mainnet / uang nyata)

Checklist ini melengkapi [rlangga-production-hazards-and-fixes.md](./rlangga-production-hazards-and-fixes.md). Centang per item sebelum menganggap worker **layak 24/7 dengan eksekusi nyata**.

Ringkasan **celah implementasi vs dokumen** (wallet stub, recovery kosong, dll.): [implementation-vs-spec.md](./implementation-vs-spec.md).

---

## 1. Konfigurasi & rahasia

- [ ] `RPC_STUB=0` untuk path yang membutuhkan konfirmasi transaksi nyata (`WaitTxConfirmed`).
- [ ] `RPC_URL` atau **`RPC_URLS`** (beberapa URL dipisah koma) ke endpoint **mainnet** yang stabil (mis. Helius) dengan **kuota** memadai — kode memutar endpoint jika respons gagal saat konfirmasi tx.
- [ ] `PUMPPORTAL_URL` / `PUMPAPI_URL` / `PUMP_NATIVE` + kunci mengarah ke **gateway atau API resmi** yang sudah diuji (bukan URL contoh / dummy).
- [ ] `SIMULATE_TRADING=0` dan `SIMULATE_ENGINE=0` untuk trading nyata.
- [ ] `REDIS_URL` menun ke instance **Redis produksi** (persistensi AOF/RDB sesuai kebijakan Anda).
- [ ] `.env` **tidak** ter-commit; kunci API / private key hanya di secret store / CI aman.
- [ ] `ENABLE_TRADING`, `MAX_DAILY_LOSS`, `MIN_BALANCE`, `MAX_DAILY_TRADES`, `LOCK_TTL_MIN` sesuai risiko dan **LOCK_TTL_MIN > durasi monitor maksimum** yang Anda izinkan (lihat hazard §5).

---

## 2. Wallet & saldo (kode saat ini masih stub)

- [ ] **Wajib sebelum live:** `internal/wallet.GetSOLBalance` terhubung ke **RPC** (atau layanan) yang mengembalikan saldo SOL wallet trading **nyata** — bukan nilai tetap 1.0.
- [ ] Verifikasi manual: nilai `MIN_BALANCE` vs saldo aktual; gate `guard.CanTrade` benar‑benar memblokir jika saldo tidak cukup.
- [ ] Private key / signer hanya dipakai di proses yang aman (bukan log, bukan repo).

---

## 3. Recovery & posisi orphan

- [ ] **Wajib sebelum mengandalkan recovery 24/7:** `internal/rpc.GetWalletTokens` (atau setara) mengembalikan **token SPL** di wallet untuk **force sell** — bukan slice kosong permanen.
- [ ] Uji: ada token sisa kecil → recovery mencoba SELL sesuai `MIN_DUST` / hazard §3.
- [ ] Pastikan `sellguard` + Redis aktif agar monitor vs recovery **tidak** double-sell (hazard §2).

---

## 4. Stream & beban

- [ ] `PUMP_WS_URL` + `PUMP_WS_AUTO_HANDLE` sesuai sumber data; pahami **rate** event (banyak mint/detik).
- [ ] Pertimbangkan **batas konkurensi** (antrian / semaphore) jika stream sangat ramai — hindari OOM dan ban API.
- [ ] `QUOTE_INTERVAL_MS`, timeout HTTP, dan `QUOTE_MAX_AGE_MS` selaras dengan latensi jaringan Anda (hazard §4).

---

## 5. Pengujian sebelum nominal penuh

Ikuti urutan di **[testing-and-release.md](./testing-and-release.md)** (fmt → vet → tes parity CI → staging → nominal kecil).

- [ ] **`make ci`** lokal hijau (sama gate dengan GitHub Actions).
- [ ] Staging: Redis + `.env` realistis; **`SIMULATE_ENGINE`** / **`SIMULATE_TRADING`** atau nominal minimal; **`RPC_STUB=0`** jika menguji konfirmasi tx.
- [ ] Opsional parity penuh CI: **`make test-race`** (perlu gcc).
- [ ] Skenario restart worker — Redis (trades, guard) sesuai ekspektasi.

---

## 6. Operasi 24/7

- [ ] Proses dijalankan dengan **supervisor** (systemd, Docker restart policy, k8s, dll.) + restart otomatis jika crash.
- [ ] Log dikumpulkan (file terpusat / Loki / CloudWatch) dan ada **alert** untuk error berulang atau Redis down.
- [ ] Cadangan atau prosedur restore **Redis** jika menjadi sumber kebenaran untuk statistik penting.
- [ ] Telegram / alert lain (`TELEGRAM_*`) dikonfigurasi jika dipakai untuk kill switch & laporan.

---

## 7. Tindakan cepat darurat

- [ ] Tahu cara set `ENABLE_TRADING=false` atau stop proses tanpa merusak state.
- [ ] Tahu cara `go run ./cmd/reset-pnl` (atau setara) **hanya** jika Anda sengaja mengosongkan statistik trade di Redis — tidak menggantikan matikan trading.

---

## Status singkat (kode saat ini)

| Komponen | Siap live? |
|----------|------------|
| Daemon worker + WS + recovery loop | Struktur **cocok** jalan terus |
| Gate guard + lock + idempotency + sellguard | **Ada**, perlu Redis & konfig benar |
| Saldo SOL via RPC | **Belum** — masih stub di `wallet` |
| Daftar token wallet untuk recovery | **Belum** — stub di `GetWalletTokens` |
| Eksekusi pump + RPC | **Tergantung** `.env` dan gateway Anda |

**Kesimpulan:** anggap checklist di atas sebagai **gate**; tanpa baris “wajib” di bagian wallet & recovery terisi, sistem **belum** layak dianggap siap live 24/7 untuk ukuran penuh.
