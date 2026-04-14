# Implementasi vs spesifikasi (1:1)

Dokumen ini mencatat **perbedaan** antara dokumen desain (blueprint, PR-001–005, `rlangga-repo-structure.md`) dan **perilaku kode saat ini** di repositori. Tujuannya: menghindari asumsi “sudah 1:1” tanpa verifikasi.

---

## 1. Wallet & saldo SOL

| Area | Spesifikasi / harapan umum | Implementasi saat ini |
|------|----------------------------|------------------------|
| Saldo SOL untuk guard (`MIN_BALANCE`, dll.) | Membaca saldo wallet nyata dari RPC | `internal/wallet/wallet.go`: **`GetSOLBalance` mengembalikan `1.0` stub** kecuali `BalanceHook` diset (tes). **Guard produksi berdasarkan saldo nyata belum terhubung ke RPC wallet.** |

**Implikasi:** `CanTrade` dan pemblokiran BUY karena saldo bisa **tidak mencerminkan** saldo on-chain sampai integrasi RPC saldo selesai.

---

## 2. Recovery: daftar token wallet

| Area | Spesifikasi | Implementasi saat ini |
|------|-------------|------------------------|
| `RecoverAll` memindai token di wallet | RPC `getTokenAccountsByOwner` (atau setara) → jual sisa | `internal/rpc/rpc.go` — **`GetWalletTokens` mengembalikan slice kosong`** dengan komentar `TODO` (kecuali `WalletTokensHook` di tes). **Loop recovery berjalan, tetapi tidak ada token yang terdeteksi dari jaringan.** |

**Implikasi:** Recovery **force-sell** posisi nyangkut **bergantung** pada integrasi ini; sampai itu, perilaku “scan wallet” di dokumen = **no-op** secara default.

---

## 3. Konfirmasi transaksi RPC

| Area | Catatan lama di repo | Implementasi saat ini |
|------|----------------------|-------------------------|
| `WaitTxConfirmed` | Beberapa cuplikan historis menyebut perilaku stub | Polling **`getSignatureStatuses`** ke endpoint `RPC_URL` / rotasi **`RPC_URLS`** (failover). **`RPC_STUB=1`** tetap memaksa konfirmasi “sukses” tanpa jaringan (sesuai uji). |

---

## 4. PR-002 / `HandleMint`: `time.Sleep` vs monitor

| Area | PR-001 (cuplikan) | Implementasi saat ini |
|------|-------------------|------------------------|
| Alur setelah BUY | `time.Sleep` tetap, TODO PR-002 | **Sudah** memakai **`monitor.MonitorPositionWithBot`** + exit adaptif (kecuali mode `SIMULATE_TRADING` stream-only atau cabang khusus). **TODO di teks PR-001 perlu dibaca sebagai historis** — lihat `internal/app/app.go` + `internal/monitor`. |

---

## 5. PR-004: “beberapa worker paralel”

| Area | Wording di PR-004 | Implementasi saat ini |
|------|-------------------|------------------------|
| Paralelisme | Throughput via beberapa worker | **Satu proses** `cmd/worker` memuat **`orchestrator`** (round-robin **beberapa profil bot**). **Scaling horizontal** = menjalankan **lebih dari satu replika container** worker yang **berbagi Redis** (lock global), bukan thread paralel dalam satu handler. |
| Konfigurasi bot | `BotConfig` | **`BOTS_JSON`** di env (opsional); kosong = dua profil default di `internal/bot/bot.go`. |

---

## 6. Quote & eksekusi Pump

| Area | Cuplikan `rlangga-repo-structure.md` | Implementasi saat ini |
|------|----------------------------------------|------------------------|
| `GetSellQuote` | Placeholder angka tetap | **HTTP** ke Pump Portal/API + mode **`PUMP_NATIVE`** (`internal/quote`, `internal/pumpnative`). Bukan stub tunggal. |
| BUY/SELL | Contoh pseudocode | **`internal/executor`** memanggil integrasi nyata atau stub tergantung config + **`SIMULATE_ENGINE`**. |

---

## 7. Dokumen `rlangga-repo-structure.md`

Cuplikan kode di beberapa bagian (mis. `app.go`, `executor.go`, `quote.go`) adalah **ilustrasi arsitektur awal**, bukan salinan baris-per-baris dari tree saat ini. Struktur direktori dan paket (`pumpws`, `pumpnative`, `redisx`, `sellguard`, `cmd/reset-pnl`, dll.) **lebih lengkap** di repositori aktual.

**Rujukan kanonik perilaku:** kode di `internal/` + [rlangga-env-contract.md](./rlangga-env-contract.md) + `.env.example`.

---

## 8. Variabel lingkungan

Semua nama yang dipakai parser (`internal/config`) tercatat di **[rlangga-env-contract.md](./rlangga-env-contract.md)** (termasuk §7a–7d). Jika sebuah PR lama tidak menyebut `RPC_URLS`, `SIMULATE_ENGINE`, `BOTS_JSON`, dll., **utamakan kontrak env**, bukan teks PR saja.

---

## 9. Urutan `HandleMint` (PR-004 / PR-005 vs kode)

| Sumber | Catatan |
|--------|---------|
| Cuplikan historis di PR-005 §8 (hanya guard → idempotency → lock) | **Kurang** langkah filter opsional (initial buy WSS, dll.) — **jangan dipakai sebagai urutan tunggal**. |
| Implementasi `internal/app/app.go` | **Kanonik:** `CanTrade` → *(opsional)* `FILTER_REQUIRE_INITIAL_BUY` → *(opsional)* `FILTER_ANTI_RUG` + `filter.AllowMint` → `idempotency` → `lock` → … |

**PR-004 §5** dan **PR-005 §8** sudah diperbarui agar selaras dengan urutan di atas. Blueprint §1 (*eksekusi cepat*) + filter on-chain dijabarkan di [filter-rug-honeypot.md](./filter-rug-honeypot.md) dan env §7c.1.

---

## Ringkas

| Siap produksi penuh “seperti blueprint” | Bagian yang masih stub / perlu integrasi |
|----------------------------------------|------------------------------------------|
| Lock, idempotency, monitor, exit, guard config, filter opsional, failover RPC status, mode simulasi | **Saldo SOL wallet**, **daftar token SPL di wallet** untuk recovery |

Untuk kesiapan **simulasi stream + paper engine**, lihat [simulation-readiness.md](./simulation-readiness.md).
