# Kontrak variabel lingkungan RLANGGA

**Jenjang dokumen:** referensi tunggal — menggabungkan konfigurasi yang tersebar di [rlangga-blueprint-v2.md](./rlangga-blueprint-v2.md), PR-001–PR-005, [rlangga-full-stack.md](./rlangga-full-stack.md), dan [rlangga-production-hazards-and-fixes.md](./rlangga-production-hazards-and-fixes.md).

**Aturan:** nilai di bawah adalah **kontrak nama + makna**; contoh angka bisa disesuaikan deployment. Yang bertanda *opsional* boleh ditunda sampai fitur terkait diimplementasi.

---

## 1. Infrastruktur & data

| Variabel | Wajib | Contoh | Sumber | Keterangan |
|----------|-------|--------|--------|------------|
| `REDIS_URL` | Ya | `redis:6379` | PR-001, full-stack | Host:port untuk `go-redis`; di Compose gunakan nama service `redis` |
| `RPC_URL` | Ya* | URL HTTPS Helius / penyedia | PR-001 | Endpoint utama RPC Solana; **alternatif failover:** `RPC_URLS` (§7a). *Jika `RPC_STUB=1`, URL boleh kosong untuk uji tanpa jaringan.* |

---

## 2. Eksekusi & API

| Variabel | Wajib | Contoh | Sumber | Keterangan |
|----------|-------|--------|--------|------------|
| `PUMP_NATIVE` | Disarankan | `1` | `pumpnative` | `1` = jalur resmi Lightning (`/api/trade`) + JSON PumpAPI (`api.pumpapi.io`). |
| `PUMPPORTAL_URL` | Ya | `https://pumpportal.fun/api/trade` | PR-001 + implementasi | Default internal sama jika kosong (native). |
| `PUMPAPI_URL` | Disarankan | `https://api.pumpapi.io` | PR-001 + implementasi | Default internal sama jika kosong (native). |
| `TRADE_SIZE` | Ya | `0.1` | PR-001 | Ukuran masuk dalam SOL (sesuai integrasi) |
| `TIMEOUT_MS` | Ya | `1500` | PR-001 | Timeout permintaan HTTP / RPC klien |

Rujukan host: [PumpPortal Trading API](https://pumpportal.fun/trading-api/), [PumpAPI](https://pumpapi.io/). WebSocket: `wss://pumpportal.fun/api/data`, `wss://stream.pumpapi.io` (lihat §7c).

---

## 3. Recovery & loop

| Variabel | Wajib | Contoh | Sumber | Keterangan |
|----------|-------|--------|--------|------------|
| `RECOVERY_INTERVAL` | Ya | `10` | PR-001 | Detik antar iterasi `RecoverAll`; naikkan jika RPC terbebani (hazards §8) |

---

## 4. Adaptive exit & monitor (PR-002)

| Variabel | Wajib | Contoh | Sumber | Keterangan |
|----------|-------|--------|--------|------------|
| `GRACE_SECONDS` | Ya | `2` | PR-002 | Jendela noise awal |
| `MIN_HOLD` | Ya | `5` | PR-002 | Detik minimum sebelum TP penuh (selain grace) |
| `MAX_HOLD` | Ya | `15` | PR-002 | Batas hold absolut |
| `TP_PERCENT` | Ya | `7` | PR-002 | Take profit (%) |
| `SL_PERCENT` | Ya | `5` | PR-002 | Stop loss (%) |
| `PANIC_SL` | Ya | `8` | PR-002 | Panic cut (%) |
| `MOMENTUM_DROP` | Ya | `2.5` | PR-002 | Ambang penurunan dari peak PnL (%) |
| `QUOTE_INTERVAL_MS` | Ya | `500` | PR-002 | Interval poll quote di monitor |

---

## 5. Profit guard & gate BUY (PR-005)

| Variabel | Wajib | Contoh | Sumber | Keterangan |
|----------|-------|--------|--------|------------|
| `MAX_DAILY_LOSS` | Ya | `0.5` | PR-005 | Akumulasi rugi harian (SOL) sebelum kill switch |
| `MIN_BALANCE` | Ya | `0.2` | PR-005 | Saldo SOL minimum untuk mengizinkan BUY |
| `ENABLE_TRADING` | Ya | `true` | PR-005 | `false` = hentikan BUY baru (maintenance) |

---

## 6. Governance (blueprint) — wajib di `guard`

| Variabel | Wajib | Contoh | Sumber | Keterangan |
|----------|-------|--------|--------|------------|
| `TZ` | Disarankan | `Asia/Jakarta` | Blueprint §6 | Zona waktu untuk jendela jam aktif |
| `ACTIVE_START_HOUR` | Disarankan | `20` | Blueprint §6 | Jam mulai BUY (0–23), WIB jika `TZ=Asia/Jakarta` |
| `ACTIVE_END_HOUR` | Disarankan | `2` | Blueprint §6 | Jam akhir rentang yang melewati tengah malam — gunakan logika OR untuk jam (lihat hazards §7) |
| `MAX_DAILY_TRADES` | Disarankan | `50` | Blueprint §7 | Kuota trade harian; counter Redis; cek di `CanTrade` (bukan increment pada BUY gagal — hazards §6) |

---

## 7. Operasional & keamanan (opsional / hazards)

| Variabel | Wajib | Contoh | Sumber | Keterangan |
|----------|-------|--------|--------|------------|
| `MIN_DUST` | Opsional | `0.0001` | Hazards §3 | Abaikan sisa saldo di bawah ambang (SOL) |
| `QUOTE_MAX_AGE_MS` | Opsional | `1000` | Hazards §4 | Lewati iterasi monitor jika quote lebih tua dari ini |
| `LOCK_TTL_MIN` | Opsional | `12` | Hazards §5 | TTL lock mint (menit); atau gunakan refresh lock |
| `STALE_BALANCE_WAIT_MS` | Opsional | `1500` | Hazards §11 | Jeda sebelum cek saldo pasca-SELL |

---

## 7a. RPC lanjutan, stub, failover, paper (mainnet)

| Variabel | Wajib | Contoh | Sumber | Keterangan |
|----------|-------|--------|--------|------------|
| `RPC_URLS` | Opsional | `https://a,https://b` | Implementasi | Beberapa URL dipisah **koma**; dipakai untuk failover pemanggilan `getSignatureStatuses` (konfirmasi tx). Jika di-set, daftar ini yang dipakai; `RPC_URL` tetap dipakai sebagai input fallback saat membangun daftar. |
| `RPC_STUB` | Uji lokal | `1` | Implementasi | `1` / `true`: RPC disimulasikan (`WaitTxConfirmed` selalu sukses tanpa jaringan). **Produksi swap nyata:** `0` + `RPC_URL` atau `RPC_URLS` valid. |
| `PAPER_TRADE` | Opsional | `0` / `1` | Implementasi | Flag uji dengan **RPC nyata** (bukan stub); wajib `RPC_STUB=0`. Deployment target repo ini: **mainnet** (bukan devnet). |
| `HELIUS_API_KEY` | Opsional | secret | Implementasi | Jika `PAPER_TRADE=1` dan `RPC_URL` kosong: URL **Helius mainnet** dibangun otomatis. Untuk RPC lain, set `RPC_URL` / `RPC_URLS` eksplisit. |

---

## 7b. Pump native (Lightning) & quote terpisah

| Variabel | Wajib | Contoh | Sumber | Keterangan |
|----------|-------|--------|--------|------------|
| `PUMP_NATIVE` | Opsional | `1` | `pumpnative` | Mengaktifkan jalur trade/quote JSON resmi (bukan hanya URL legacy `/buy`). |
| `PUMPPORTAL_API_KEY` | Jika dipakai API | secret | Pump | Header/kunci Portal. |
| `PUMP_PRIVATE_KEY` | Disarankan PumpAPI | secret base58 | PumpAPI | Field JSON `privateKey` — jangan commit; lihat dokumen PumpAPI. |
| `PUMP_WALLET_PUBLIC_KEY` | **Wajib** jika `RPC_STUB=0` dan `ENABLE_TRADING=true` | pubkey | Pump + RPC | Saldo SOL (`getBalance`) dan token SPL (`getTokenAccountsByOwner`); juga field `publicKey` PumpAPI legacy jika tanpa `PUMP_PRIVATE_KEY`. |
| `PUMPPORTAL_QUOTE_URL` | Opsional | URL basis | Implementasi | Basis `/quote` jika berbeda dari `PUMPPORTAL_URL`. |
| `PUMPAPI_QUOTE_URL` | Opsional | URL basis | Implementasi | Idem untuk PumpAPI. |
| `PUMPAPI_QUOTE_MINT` | Opsional | mint | PumpAPI | Pool non-SOL quote (mis. USDC). |
| `PUMPAPI_POOL_ID` | Opsional | string | PumpAPI | Trade dari pool tertentu. |
| `PUMPAPI_GUARANTEED_DELIVERY` | Opsional | `0`/`1` | PumpAPI | `guaranteedDelivery` — rebroadcast ~10s. |
| `PUMPAPI_JITO_TIP` | Opsional | SOL | PumpAPI | `jitoTip` (min ~0.0002 untuk Jito). |
| `PUMPAPI_MAX_PRIORITY_FEE` | Opsional | SOL | PumpAPI | `maxPriorityFee` untuk mode `auto` / `auto-*`. |
| `PUMPAPI_PRIORITY_FEE_MODE` | Opsional | `auto`, `auto-95`, … | PumpAPI | Menggantikan angka `PUMP_PRIORITY_FEE` di body PumpAPI bila di-set. |
| `PUMP_SLIPPAGE` | Opsional | `10` | Implementasi | Slippage Portal (default 10). |
| `PUMPAPI_SLIPPAGE` | Opsional | `20` | Implementasi | Slippage PumpAPI (default 20). |
| `PUMP_PRIORITY_FEE` | Opsional | `0.00005` | Implementasi | Priority fee (SOL); PumpAPI: bisa diganti string lewat `PUMPAPI_PRIORITY_FEE_MODE`. |

---

## 7c. WebSocket Pump & mode simulasi

| Variabel | Wajib | Contoh | Sumber | Keterangan |
|----------|-------|--------|--------|------------|
| `PUMP_WS_URL` | Opsional | `wss://stream.pumpapi.io` | Implementasi | **Disarankan** satu sumber PumpAPI; alternatif PumpPortal `wss://pumpportal.fun/api/data` — lihat [wss-data-for-filters.md](./wss-data-for-filters.md). |
| `PUMP_WS_FALLBACK_URL` | Opsional | kosong | Implementasi | Stream kedua paralel (hindari jika tidak perlu); dedupe mint. |
| `PUMP_WS_SUBSCRIBE_JSON` | Opsional | JSON array | Implementasi | Khusus PumpPortal; PumpAPI stream sering **tanpa** subscribe manual. |
| `PUMP_WS_FALLBACK_SUBSCRIBE_JSON` | Opsional | JSON array | Implementasi | Subscribe khusus fallback; kosong = pakai `PUMP_WS_SUBSCRIBE_JSON`. |
| `PUMP_WS_AUTO_HANDLE` | Opsional | `false` | Implementasi | `true`: parse mint dari pesan → `HandleMint` (risiko di produksi). |
| `SIMULATE_TRADING` | Opsional | `0` / `1` | Implementasi | Stream aktif tetapi tanpa BUY/monitor penuh (hanya log/dedupe), kecuali `SIMULATE_ENGINE`. |
| `SIMULATE_ENGINE` | Opsional | `0` / `1` | Implementasi | Alur guard → lock → BUY/SELL **tanpa** tx on-chain; monitor + exit adaptif; quote HTTP atau sintetis. |
| `SIMULATE_SYNTH_AMPLITUDE_PCT` | Opsional | | Implementasi | Amplitudo osilasi PnL paper (0 = default internal). |
| `SIMULATE_SYNTH_PERIOD_SEC` | Opsional | | Implementasi | Periode osilasi (0 = default). |
| `SIMULATE_SYNTH_DRIFT_PCT` | Opsional | | Implementasi | Bias % (boleh negatif). |

Reset Redis + profil tuning (quote-first vs campuran sintetis): [sim-engine-tuning.md](./sim-engine-tuning.md).

Ringkasan field JSON / subscribe untuk **filter** (bukan hanya mint): [wss-data-for-filters.md](./wss-data-for-filters.md).

### 7c.1 Filter on-chain (anti-rug / honeypot)

| Variabel | Wajib | Contoh | Keterangan |
|----------|-------|--------|------------|
| `FILTER_ANTI_RUG` | Opsional | `0` / `1` | `1` = panggil RPC (`getAccountInfo`, dll.) sebelum BUY — lihat [filter-rug-honeypot.md](./filter-rug-honeypot.md). |
| `FILTER_REJECT_FREEZE_AUTHORITY` | Opsional | `1` | Tolak mint dengan freeze authority. |
| `FILTER_REJECT_MINT_AUTHORITY` | Opsional | `0` | Tolak jika mint authority masih ada. |
| `FILTER_MAX_TOP_HOLDER_PCT` | Opsional | `0` | Default **0** (cepat, tanpa RPC largest-accounts). Set mis. `5` untuk tolak jika holder terbesar **>** 5%; perlu `FILTER_ANTI_RUG=1`. |
| `FILTER_RPC_FAIL_OPEN` | Opsional | `1` | `1` = error RPC → loloskan BUY (hati-hati). |
| `FILTER_REQUIRE_INITIAL_BUY` | Opsional | `0` | `1` = wajibkan field `initialBuy` di payload WSS (`ParseStreamEvent`); mint hanya dari `ExtractMint` atau tanpa `initialBuy` → BUY di-skip. |
| `FILTER_MIN_ENTRY_SOL_IN_POOL` | Opsional | `0` | Minimal `solInPool` dari payload AMM (mis. `pump-amm`). `0` = off. Hanya mem-filter jika field ada di payload. |
| `FILTER_MIN_BURNED_LIQUIDITY_PCT` | Opsional | `0` | Minimal burned liquidity (0–100) dari field `burnedLiquidity` (mis. `\"100%\"`). `0` = off. |
| `FILTER_REJECT_POOL_CREATED_BY_CUSTOM` | Opsional | `0` | `1` = tolak jika `poolCreatedBy=custom` pada payload (pool manual; lebih berisiko). |

### 7c.2 Filter payload WebSocket (pra-`HandleMint`)

Hanya dievaluasi jika `PUMP_WS_AUTO_HANDLE=1` **dan** setidaknya satu variabel di bawah di-set (bukan nol dan tidak kosong). Parsing: `internal/pumpws.ParseStreamEvent` — lihat [wss-data-for-filters.md](./wss-data-for-filters.md).

**Profil ringkas (hanya pool):** cukup **`FILTER_WSS_POOL`** (mis. `pump` atau `pump,pump-amm`); variabel WSS lainnya boleh kosong — gate yang jalan hanya pengecekan field `pool` pada payload.

| Variabel | Wajib | Contoh | Keterangan |
|----------|-------|--------|------------|
| `FILTER_WSS_ALLOW_TX_TYPES` | Opsional | `create,buy` | Daftar dipisah koma; **case-insensitive**. Jika di-set, `txType`/`type`/`event` dari JSON harus cocok salah satu; kosong di payload → ditolak. |
| `FILTER_WSS_DENY_TX_TYPES` | Opsional | `sell` | Tolak jika tipe cocok salah satu entri. |
| `FILTER_WSS_ALLOW_METHODS` | Opsional | `subscribenewtoken` | Jika di-set, field `method`/`channel` harus cocok (payload tanpa field → ditolak). |
| `FILTER_WSS_MIN_SOL` | Opsional | `0` | Minimal nilai SOL dari field numerik (`solAmount`, `sol`, …) atau `lamports`/1e9; `0` = off. |
| `FILTER_WSS_MAX_SOL` | Opsional | `0` | Maksimum; `0` = off. Harus `MIN` ≤ `MAX` jika keduanya positif. |
| `FILTER_WSS_POOL` | Opsional | `pump` | Daftar dipisah koma; jika di-set, field `pool` dari payload (root atau nested) harus cocok salah satu. |
| `FILTER_WSS_MIN_MARKET_CAP_SOL` | Opsional | `0` | Minimal `marketCapSol` (angka dari JSON); `0` = off. |
| `FILTER_WSS_MAX_MARKET_CAP_SOL` | Opsional | `0` | Maksimum; `0` = off. Harus `MIN` ≤ `MAX` jika keduanya positif. |

---

## 7d. Multi-bot (PR-004)

| Variabel | Wajib | Contoh | Sumber | Keterangan |
|----------|-------|--------|--------|------------|
| `BOTS_JSON` | Opsional | JSON array | PR-004 / kode | Daftar `BotConfig` (nama, hold, TP/SL, dll.). Kosong = profil bawaan **bot-10s** + **bot-15s** di kode. |

---

## 8. Laporan (PR-003 / Telegram)

| Variabel | Wajib | Contoh | Sumber | Keterangan |
|----------|-------|--------|--------|------------|
| `TELEGRAM_BOT_TOKEN` | Jika pakai TG | secret | Full-stack | Jangan commit ke repo |
| `TELEGRAM_CHAT_ID` | Jika pakai TG | id channel/user | PR-003 | Tujuan alert / summary |
| `REPORT_EVERY_N_TRADES` | Opsional | `5` | PR-003 | Pemicu ringkasan |
| `REPORT_INTERVAL_MIN` | Opsional | `30` | PR-003 | Pemicu interval (menit) |
| `REPORT_LOAD_RECENT` | Opsional | `200` | Implementasi | Batas jumlah trade terbaru yang dimuat untuk agregat/laporan (di kode: `ReportMaxTrades`). |

---

## 9. Contoh `.env` (gabungan)

```env
# Infra
REDIS_URL=redis:6379
RPC_URL=https://...

# Eksekusi (URL resmi — lihat §2)
PUMP_NATIVE=1
PUMPPORTAL_URL=https://pumpportal.fun/api/trade
PUMPAPI_URL=https://api.pumpapi.io
PUMPPORTAL_API_KEY=...
TRADE_SIZE=0.1
TIMEOUT_MS=1500

# Recovery
RECOVERY_INTERVAL=10

# Exit (PR-002)
GRACE_SECONDS=2
MIN_HOLD=5
MAX_HOLD=15
TP_PERCENT=7
SL_PERCENT=5
PANIC_SL=8
MOMENTUM_DROP=2.5
QUOTE_INTERVAL_MS=500

# Guard (PR-005)
MAX_DAILY_LOSS=0.5
MIN_BALANCE=0.2
ENABLE_TRADING=true

# Governance (blueprint)
TZ=Asia/Jakarta
ACTIVE_START_HOUR=20
ACTIVE_END_HOUR=2
MAX_DAILY_TRADES=50

# Filter on-chain (§7c.1) — semua opsional; bawaan konsisten dengan internal/config
# FILTER_ANTI_RUG=0
# FILTER_REJECT_FREEZE_AUTHORITY=1
# FILTER_REJECT_MINT_AUTHORITY=0
# FILTER_MAX_TOP_HOLDER_PCT=0
# FILTER_RPC_FAIL_OPEN=1
# FILTER_REQUIRE_INITIAL_BUY=0

# Filter payload WSS (§7c.2; butuh PUMP_WS_AUTO_HANDLE=1)
# FILTER_WSS_ALLOW_TX_TYPES=create
# FILTER_WSS_DENY_TX_TYPES=
# FILTER_WSS_ALLOW_METHODS=
# FILTER_WSS_MIN_SOL=0
# FILTER_WSS_MAX_SOL=0
# FILTER_WSS_POOL=
# FILTER_WSS_MIN_MARKET_CAP_SOL=0
# FILTER_WSS_MAX_MARKET_CAP_SOL=0

# Opsional
# MIN_DUST=0.0001
# QUOTE_MAX_AGE_MS=1000
# LOCK_TTL_MIN=12
# TELEGRAM_BOT_TOKEN=
# TELEGRAM_CHAT_ID=
```

---

## 10. Docker Compose

`environment` pada service `worker` harus memuat minimal set **Wajib** dari §1–5 dan §6 sesuai kebutuhan rilis. Lihat juga [rlangga-full-stack.md](./rlangga-full-stack.md) §3.

---

## Rujukan cepat PR

| PR | Variabel utama |
|----|----------------|
| PR-001 | `REDIS_URL`, `RPC_URL`, `RPC_URLS`, `RPC_STUB`, `PUMPPORTAL_URL`, `PUMPAPI_URL`, `TRADE_SIZE`, `TIMEOUT_MS`, `RECOVERY_INTERVAL` |
| PR-002 | `GRACE_SECONDS` … `QUOTE_INTERVAL_MS` |
| PR-003 | kunci Redis + opsional Telegram / interval laporan + `REPORT_LOAD_RECENT` |
| PR-004 | `BOTS_JSON` (opsional; override profil bawaan) |
| PR-005 | `MAX_DAILY_LOSS`, `MIN_BALANCE`, `ENABLE_TRADING`, `MAX_DAILY_TRADES` |

---

## 11. Operasional Redis (bukan env worker)

| Mekanisme | Keterangan |
|-----------|------------|
| `go run ./cmd/reset-pnl` | Reset agregat PnL / trade log / state laporan di Redis (lihat README dan kode perintah). |

---

*Variabel di §7a–7d mencerminkan **implementasi saat ini**; cuplikan historis di PR lama bisa tidak menyebutnya — utamakan tabel ini + [.env.example](../.env.example). Detail penyimpangan perilaku vs blueprint: [implementation-vs-spec.md](./implementation-vs-spec.md).*
