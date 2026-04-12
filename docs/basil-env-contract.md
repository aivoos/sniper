# Kontrak variabel lingkungan BASIL

**Jenjang dokumen:** referensi tunggal — menggabungkan konfigurasi yang tersebar di [basil-blueprint-v2.md](./basil-blueprint-v2.md), PR-001–PR-005, [basil-full-stack.md](./basil-full-stack.md), dan [basil-production-hazards-and-fixes.md](./basil-production-hazards-and-fixes.md).

**Aturan:** nilai di bawah adalah **kontrak nama + makna**; contoh angka bisa disesuaikan deployment. Yang bertanda *opsional* boleh ditunda sampai fitur terkait diimplementasi.

---

## 1. Infrastruktur & data

| Variabel | Wajib | Contoh | Sumber | Keterangan |
|----------|-------|--------|--------|------------|
| `REDIS_URL` | Ya | `redis:6379` | PR-001, full-stack | Host:port untuk `go-redis`; di Compose gunakan nama service `redis` |
| `RPC_URL` | Ya | URL HTTPS Helius / penyedia | PR-001 | Endpoint RPC Solana (validasi tx + wallet) |

---

## 2. Eksekusi & API

| Variabel | Wajib | Contoh | Sumber | Keterangan |
|----------|-------|--------|--------|------------|
| `PUMPPORTAL_URL` | Ya | URL primer | PR-001 | Eksekusi / quote primer |
| `PUMPAPI_URL` | Disarankan | URL fallback | PR-001 | Fallback jika primer gagal |
| `TRADE_SIZE` | Ya | `0.1` | PR-001 | Ukuran masuk dalam SOL (sesuai integrasi) |
| `TIMEOUT_MS` | Ya | `1500` | PR-001 | Timeout permintaan HTTP / RPC klien |

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

## 8. Laporan (PR-003 / Telegram)

| Variabel | Wajib | Contoh | Sumber | Keterangan |
|----------|-------|--------|--------|------------|
| `TELEGRAM_BOT_TOKEN` | Jika pakai TG | secret | Full-stack | Jangan commit ke repo |
| `TELEGRAM_CHAT_ID` | Jika pakai TG | id channel/user | PR-003 | Tujuan alert / summary |
| `REPORT_EVERY_N_TRADES` | Opsional | `5` | PR-003 | Pemicu ringkasan |
| `REPORT_INTERVAL_MIN` | Opsional | `30` | PR-003 | Pemicu interval (menit) |

---

## 9. Contoh `.env` (gabungan)

```env
# Infra
REDIS_URL=redis:6379
RPC_URL=https://...

# Eksekusi
PUMPPORTAL_URL=https://...
PUMPAPI_URL=https://...
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

# Opsional
# MIN_DUST=0.0001
# QUOTE_MAX_AGE_MS=1000
# LOCK_TTL_MIN=12
# TELEGRAM_BOT_TOKEN=
# TELEGRAM_CHAT_ID=
```

---

## 10. Docker Compose

`environment` pada service `worker` harus memuat minimal set **Wajib** dari §1–5 dan §6 sesuai kebutuhan rilis. Lihat juga [basil-full-stack.md](./basil-full-stack.md) §3.

---

## Rujukan cepat PR

| PR | Variabel utama |
|----|----------------|
| PR-001 | `REDIS_URL`, `RPC_URL`, `PUMPPORTAL_URL`, `PUMPAPI_URL`, `TRADE_SIZE`, `TIMEOUT_MS`, `RECOVERY_INTERVAL` |
| PR-002 | `GRACE_SECONDS` … `QUOTE_INTERVAL_MS` |
| PR-003 | kunci Redis + opsional Telegram / interval laporan |
| PR-004 | tidak menambah env wajib baru (profil di kode atau file terpisah) |
| PR-005 | `MAX_DAILY_LOSS`, `MIN_BALANCE`, `ENABLE_TRADING` |

---

*Jika nama variabel di kode menyimpang dari tabel ini, utamakan dokumen ini sebagai kontrak untuk diselaraskan pada refactor berikutnya.*
