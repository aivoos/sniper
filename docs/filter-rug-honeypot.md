# Filter anti-rug & indikator honeypot (on-chain)

## Initial buy dari stream (WSS)

Jika `FILTER_REQUIRE_INITIAL_BUY=1`, hanya sinyal yang punya field **`initialBuy`** di payload WebSocket (setelah `ParseStreamEvent`) yang diproses — **sebelum** filter RPC dan idempotency. Mint yang hanya ter-parse lewat `ExtractMint` (tanpa snapshot lengkap) atau event tanpa `initialBuy` akan di-skip.

Worker memanggil **`filter.AllowMint`** sebelum **idempotency** dan **BUY** ketika `FILTER_ANTI_RUG=1`. Data diambil dari **RPC Solana** yang sama dengan konfigurasi (`RPC_URL` / `RPC_URLS[0]`, mis. Helius).

**Ini bukan jaminan anti-rug.** Skema token dan MEV berubah; selalu gabungkan dengan **ukuran posisi**, **exit PnL**, dan **batas harian**.

Selaras [blueprint §1](./rlangga-blueprint-v2.md) (*eksekusi cepat*) dan kontrak env ([rlangga-env-contract.md](./rlangga-env-contract.md) §7c.1): bawaan `FILTER_MAX_TOP_HOLDER_PCT=0` **tidak** memanggil RPC `getTokenLargestAccounts`; set nilai lebih besar dari nol hanya jika kamu sengaja mengaktifkan cek konsentrasi.

---

## Yang diperiksa

| Pemeriksaan | Makna |
|-------------|--------|
| Akun mint ada | Mint ter-deploy. |
| Owner = program SPL Token (legacy atau Token-2022) | Bukan akun sembarang. |
| Data mint ≥ 82 byte (layout SPL klasik) | Struktur mint standar untuk field berikut. |
| `is_initialized`, `supply` > 0 | Token aktif dan ada supply. |
| `decimals` tidak absurd (>18) | Heuristik sederhana. |
| **Freeze authority** (opsional tolak) | Jika masih di-set, pemegang bisa dibekukan — risiko kunci jual / pola mencurigakan. |
| **Mint authority** (opsional tolak) | Masih bisa mint tambahan — risiko inflasi. |
| **Konsentrasi holder teratas** (opsional; hanya jika `FILTER_MAX_TOP_HOLDER_PCT` > 0) | `getTokenSupply` + `getTokenLargestAccounts` — tolak jika pemegang #1 memegang lebih besar dari X% supply (lihat env §7c.1). |

---

## Yang *tidak* diperiksa (batasan)

- Simulasi jual penuh (honeypot “tidak bisa jual”) membutuhkan **simulasi transaksi** atau layanan khusus — **belum** diimplementasi.
- Token-2022 dengan **ekstensi** panjang: 82 byte pertama tetap dipakai untuk freeze/mint authority pada layout standar; edge case ekstensi eksotis bisa salah — pertimbangkan menonaktifkan filter atau verifikasi manual.
- **RPC error:** perilaku diatur `FILTER_RPC_FAIL_OPEN`.

---

## Variabel lingkungan

| Variabel | Default | Keterangan |
|----------|---------|------------|
| `FILTER_REQUIRE_INITIAL_BUY` | `0` | `1` = wajib `initialBuy` di payload WSS (tanpa perlu `FILTER_ANTI_RUG`). |
| `FILTER_ANTI_RUG` | `0` | `1` = aktifkan gate. |
| `FILTER_REJECT_FREEZE_AUTHORITY` | `1` | Tolak jika freeze authority ter-set. |
| `FILTER_REJECT_MINT_AUTHORITY` | `0` | Tolak jika mint authority masih ada (hati-hati: bonding pump.fun sering masih punya mint authority sampai fase tertentu). |
| `FILTER_MAX_TOP_HOLDER_PCT` | `0` | `0` = default (tanpa RPC largest-accounts). Mis. `5` = tolak jika holder terbesar **>** 5% supply. Hanya jika `FILTER_ANTI_RUG=1`. |
| `FILTER_RPC_FAIL_OPEN` | `1` | `1` = jika RPC gagal, **loloskan** (jangan blok semua trade). `0` = gagal aman (blokir jika tidak bisa verifikasi). |

Dengan `RPC_STUB=1`, filter **tidak** memanggil RPC (selalu lolos).

---

## Urutan di `HandleMint`

`guard` → **suffix `pump`** (jika diaktifkan) → **`AllowMint`** → `idempotency` → `lock` → BUY → monitor.

Kegagalan filter **tidak** mengonsumsi slot dedupe Redis.

---

## Rujukan kode

- `internal/filter/rugcheck.go`
- `internal/app/app.go` (`HandleMint`)
