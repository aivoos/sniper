# Sim engine: data nyata, tx simulasi, tuning PnL

Worker (`cmd/worker`) dengan **`SIMULATE_TRADING=0`** dan **`SIMULATE_ENGINE=1`** menjalankan pipeline sniper lengkap (guard → lock → BUY/SELL **tanpa** transaksi on-chain → monitor adaptif → `SaveTrade` / laporan), sambil membaca **stream WebSocket nyata** dan **quote HTTP** bila tersedia.

Rujukan env umum: [rlangga-env-contract.md](./rlangga-env-contract.md), stream/filter: [wss-data-for-filters.md](./wss-data-for-filters.md).

---

## Alur sesi tuning (disarankan)

1. **Reset state Redis** (trade, dedupe mint, counter laporan, stat harian guard) agar PnL laporan tidak tercampur sesi lama:
   ```bash
   make sim-reset
   ```
   Atau: `go run ./cmd/reset-pnl` (membutuhkan `REDIS_URL` sama seperti worker).

2. **Jalankan simulasi** dengan `.env` yang sudah kamu set:
   ```bash
   make sim-engine
   ```

3. Sesuaikan parameter (di bawah), henti worker (`Ctrl+C`), reset lagi bila mau sesi bersih, ulangi.

---

## Profil A — prioritas quote nyata (kurangi sintetis)

Tujuannya: monitor memakai **`RaceQuote`** (PumpPortal + PumpAPI) sebanyak mungkin; fallback osilator hanya jika quote jual tidak ada atau 0.

| Variabel | Saran |
|----------|--------|
| `SIMULATE_TRADING` | `0` |
| `SIMULATE_ENGINE` | `1` |
| `PUMPPORTAL_URL` / `PUMPAPI_URL` | Diisi (basis resmi `/api/trade` / `api.pumpapi.io`). |
| `PUMPPORTAL_QUOTE_URL` / `PUMPAPI_QUOTE_URL` | Opsional jika endpoint quote beda dari trade; harus menerima POST `/quote` + body `{"mint":...}` seperti di kode. |
| `SIMULATE_SYNTH_AMPLITUDE_PCT` | `0` (default internal dipakai hanya saat quote gagal — tetap ada osilasi cadangan; lihat `internal/quote/synthetic.go`). |
| `SIMULATE_SYNTH_PERIOD_SEC` | `0` |
| `SIMULATE_SYNTH_DRIFT_PCT` | `0` atau kosong |
| `QUOTE_INTERVAL_MS` | Lebih cepat = lebih reaktif (mis. `400`–`500`); beban HTTP naik. |
| `QUOTE_MAX_AGE_MS` | `0` = jangan anggap quote kadaluarsa; atau set ms jika ingin skip tick lama. |

Catatan: jika API tidak mengembalikan harga jual > 0 untuk mint tertentu, kode tetap bisa jatuh ke **harga sintetis** — itu batasan data, bukan bug.

---

## Profil WSS — data entry yang dipakai sniper (stream saja)

Untuk tuning **entry + exit**, sumber yang relevan dari worker adalah:

| Sumber | Dipakai untuk |
|--------|----------------|
| **Payload WSS** (PumpAPI / setelah `ParseStreamEvent`) | `initialBuy`, `marketCapSol`, `pool`, `poolId`, `txType`, `solAmount`, dll. — filter pra-BUY + kolom `entry_*` pada trade tertutup |
| **Quote HTTP** (`/quote`) | Harga jual simulasi / nyata di monitor — **bukan** dari WSS |
| **`exit=` / `ExitReason`** | Alasan keluar adaptif — dicatat di Redis + opsional SQLite |

Set `PUMP_WS_AUTO_HANDLE=true` dan pastikan `PUMP_WS_URL` (mis. `wss://stream.pumpapi.io`) agar mint diproses dengan snapshot stream.  
Mirror analitik: `TRADE_SQLITE_PATH=...` (lihat [sql/trades.sql](./sql/trades.sql)) — query SQL tanpa menggantungkan parse log stdout.

**Filter WSS:** profil paling sederhana — cukup **`FILTER_WSS_POOL=pump-amm`** (default template; AMM saja). txType/mcap/SOL boleh tidak di-set. Tambahan opsional: `FILTER_WSS_ALLOW_TX_TYPES`, `FILTER_MIN_INITIAL_BUY`, `FILTER_MIN_ENTRY_MARKET_CAP_SOL`, dll.

---

## Profil B — campuran quote + osilasi sintetis (eksplorasi perilaku exit)

Untuk melihat variasi exit/TP/SL dengan osilasi yang bisa diatur:

| Variabel | Contoh |
|----------|--------|
| `SIMULATE_ENGINE` | `1` |
| `SIMULATE_SYNTH_AMPLITUDE_PCT` | `15` |
| `SIMULATE_SYNTH_PERIOD_SEC` | `6` |
| `SIMULATE_SYNTH_DRIFT_PCT` | `0` |

PnL akan lebih dipengaruhi parameter ini ketika quote HTTP lemah atau nol.

---

## Yang ikut di-reset oleh `make sim-reset`

- Riwayat trade + dedupe idempotency (lihat `store.ClearTradesAndDedupe`).
- State laporan agregat (`report.ResetReportState`).
- Stat harian guard rugi/kuota (`guard.ResetDailyStats`).

Tidak mengubah isi `.env`.

---

## Target Makefile

| Target | Fungsi |
|--------|--------|
| `make sim-reset` | `go run ./cmd/reset-pnl` |
| `make sim-session` | reset lalu worker (satu baris untuk sesi baru) |
| `make sim-engine` | hanya worker |

---

## Analisis cepat setelah sesi

- **SQLite** (jika `TRADE_SQLITE_PATH` di-set):  
  `sqlite3 /path/trades.sqlite "SELECT exit_reason, COUNT(*), AVG(pnl_sol) FROM trades GROUP BY exit_reason;"`

- **Redis** (tanpa file SQL):  
  `redis-cli -u "$REDIS_URL" LRANGE trades:list 0 20` | jq .

- **Sampel frame WSS** (cek field yang masuk filter): `make wss-sample`
