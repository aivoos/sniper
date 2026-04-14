# RLANGGA Blueprint v2

**Status:** LOCKED — governed system  
**Versi dokumen:** 2

**Konvensi penamaan:** mengikuti [Google Java Style Guide](https://google.github.io/styleguide/javaguide.html) untuk identifier; nama file dokumen memakai `lower-kebab-case`.

**Dokumen turunan (level 2):**

| Dokumen | Isi |
|---------|-----|
| [rlangga-repo-structure.md](./rlangga-repo-structure.md) | Struktur repo `rlangga/`, `go.mod`, layout `cmd` / `internal`, Docker, dan baseline kode |
| [rlangga-full-stack.md](./rlangga-full-stack.md) | Stack end-to-end: infra, runtime, Redis, layanan eksternal, alur, observability, ops, multi-bot |
| [rlangga-pr-001-core-engine-recovery-validation.md](./rlangga-pr-001-core-engine-recovery-validation.md) | PR-001: eksekusi inti, recovery, validasi RPC, Redis lock/idempotency, DoD |
| [rlangga-pr-002-adaptive-exit-pnl.md](./rlangga-pr-002-adaptive-exit-pnl.md) | PR-002: quote, PnL, adaptive exit, monitor, integrasi worker PR-001 |
| [rlangga-pr-003-pnl-validation-analytics.md](./rlangga-pr-003-pnl-validation-analytics.md) | PR-003: trade store Redis, agregasi, metrik, loss streak, laporan Telegram |
| [rlangga-pr-004-multi-bot.md](./rlangga-pr-004-multi-bot.md) | PR-004: multi worker, BotConfig, orchestrator, lock bersama, exit per bot, scale Docker |
| [rlangga-pr-005-profit-guard.md](./rlangga-pr-005-profit-guard.md) | PR-005: daily loss, kill switch, balance guard, trade gate, alert Telegram, reset |
| [rlangga-production-hazards-and-fixes.md](./rlangga-production-hazards-and-fixes.md) | 12 hazard produksi: race BUY/state, double sell, dust, quote stale, lock TTL, kuota, midnight, RPC, orchestrator, float, balance lag, recovery vs guard |
| [rlangga-env-contract.md](./rlangga-env-contract.md) | Kontrak semua variabel lingkungan (`.env`) dalam satu tabel |
| [rlangga-test-standard.md](./rlangga-test-standard.md) | Standar pengujian Google-grade: piramida, determinisme, CI, mocking |
| [rlangga-dev-parity.md](./rlangga-dev-parity.md) | Parity lokal / CI / server; toolchain (Go, Docker, gcc untuk `-race`) |
| [implementation-vs-spec.md](./implementation-vs-spec.md) | Celah implementasi vs blueprint/PR (wallet stub, recovery RPC, cuplikan usang) |
| [filter-rug-honeypot.md](./filter-rug-honeypot.md) | Filter on-chain opsional sebelum BUY (freeze/mint authority, konsentrasi holder); env: [rlangga-env-contract.md](./rlangga-env-contract.md) §7c.1 |

---

## 0. Definisi sistem

| Aspek | Nilai |
|--------|--------|
| **Tipe** | Deterministic execution system |
| **Mode** | Event-driven + reconciliation loop |
| **Governance** | Dual guard (kuota + waktu) |
| **Objektif** | Bertahan + mengekstrak momentum jangka pendek |

---

## 1. Prinsip inti

1. **Eksekusi cepat** — via API  
2. **Validasi wajib** — via RPC  
3. **Recovery berkelanjutan** — loop  
4. **Exit adaptif** — sadar momentum  
5. **Trading terbatas** — kuota + waktu  
6. **Tidak ada posisi nyangkut** — invariant operasional  

---

## 2. Arsitektur (final)

Sistem direalisasikan sebagai **governed execution engine**: eksekusi, validasi, recovery, dan governance terikat erat; bukan sekadar bot trading otonom tanpa aturan.

---

## 3. Infrastruktur

| Komponen | Spesifikasi |
|----------|-------------|
| **VPS** | Vultr High Frequency |
| **Region** | US West (LA / Silicon Valley) |
| **Spes** | 2 vCPU / 4 GB RAM |
| **Jaringan** | Public IPv4 |
| **Runtime** | Docker |
| **Kontainer** | `worker` + `redis` |

---

## 4. Modul sistem

| Modul | Tanggung jawab |
|-------|------------------|
| **execution** | Buy / sell |
| **rpc** | Validasi + scan wallet |
| **recovery** | Startup + loop rekonsiliasi |
| **exit** | Exit adaptif |
| **monitor** | Loop posisi |
| **pnl** | Perhitungan PnL |
| **store** | Log perdagangan (append-only) |
| **aggregate** | Metrik agregat |
| **guard** | Kontrol risiko |
| **orchestrator** | Multi-bot (jika dipakai) |
| **lockIdempot** | Keamanan & idempotensi |

---

## 5. Guard system (final)

### Dual guard + risk guard

1. **Time window** — jam aktif vs luar jam  
2. **Trade quota** — batas frekuensi harian  
3. **Daily loss** — batas kerugian harian  
4. **Balance guard** — saldo cukup untuk operasi  

### Aturan

**BUY** hanya jika **semua** berikut terpenuhi:

- Dalam jam aktif  
- Kuota masih tersedia  
- Kerugian harian belum menyentuh limit  
- Balance cukup  

**SELL** — **selalu diizinkan** (prioritas keluar dari posisi).

---

## 6. Time window

| Zona | Waktu (WIB) | Perilaku |
|------|-------------|----------|
| **ACTIVE** | 20:00 – 02:00 | BUY diizinkan (jika guard lain lolos) |
| **OUTSIDE** | Di luar jendela di atas | **NO BUY** |

---

## 7. Trade quota

- `MAX_DAILY_TRADES = 50`  
- `remainingTrades = MAX_DAILY_TRADES - tradesUsedToday`  
- Jika `remainingTrades == 0` → **STOP BUY** (SELL tetap diizinkan)  

---

## 8. Model PnL

- **PnL** = `quoteSell - buyCost`  
- **Basis evaluasi:** simulasi jual (quote API) sebelum/untuk keputusan  

---

## 9. Adaptive exit engine

| Parameter | Nilai |
|-----------|--------|
| Panic SL | −8% |
| SL | −5% |
| TP | +7% |
| Momentum drop | 2–3% |
| Max hold | 15 s |

### Timing model

| Fase | Durasi | Karakter |
|------|--------|----------|
| Noise | 0–2 s | Noise |
| Profit window | 3–10 s | Jendela profit |
| Decay | 10–20 s | Peluruhan |

---

## 10. Recovery system (kritis)

### Startup

1. Scan wallet  
2. **Jual semua** posisi yang relevan (normalisasi state)  

### Continuous loop

- **Interval:** 5–10 detik (rentang desain); di beban RPC tinggi naikkan ke 10–15 detik atau adaptif — lihat [rlangga-production-hazards-and-fixes.md](./rlangga-production-hazards-and-fixes.md) §8  
- Scan wallet  
- Jika ada token yang harus dilikuidasi → **SELL paksa** (sesuai aturan produk)  

---

## 11. Position finalization

Alur setelah SELL:

1. Konfirmasi via RPC  
2. Cek balance  

- Balance **> 0** → retry sampai bersih  
- Balance **= 0** → selesai  

### Invariant

**Tidak boleh ada sisa trade** yang tidak diselesaikan (state harus konsisten: tidak “nyangkut”).

---

## 12. Analytics

- **Trade log:** append-only  
- **Metrik:** total trade, win rate, avg PnL, loss streak  

---

## 13. Reporting (Telegram)

- Ringkasan trade  
- Kill switch  
- Notifikasi kuota habis  

---

## 14. Failure handling

| Kejadian | Respons |
|----------|---------|
| Buy gagal | Fallback API (jika didesain) |
| Tx gagal | Blok validasi / jangan lanjut sembarangan |
| Sell gagal | Retry |
| State mismatch | Masuk recovery loop |

---

## 15. Self-healing

Urutan setelah gangguan (mis. restart VPS):

1. Docker restart  
2. Worker restart  
3. Recovery scan  
4. Posisi dibersihkan sampai invariant terpenuhi  

---

## 16. Strategi pengujian

| Lapisan | Fokus |
|---------|--------|
| Unit | Exit + PnL |
| Simulasi | Kurva pump / skenario |
| Live | Modal kecil |

Detail wajib (piramida 70/20/10, CI gate, determinisme): [rlangga-test-standard.md](./rlangga-test-standard.md).

---

## 17. Jaminan sistem (design goals)

- Tidak ada posisi nyangkut  
- Tidak overtrade (kuota + waktu)  
- Tidak trade di jam buruk (NO BUY di luar jendela)  
- Survive restart (recovery)  
- Deterministik dalam batas desain (event + loop rekonsiliasi)  

---

## 18. Inti desain

Sistem ini tidak hanya mengoptimalkan profit mentah, tetapi **menghindari kondisi buruk secara sistematis** lewat governance, validasi, dan recovery.

---

## Kunci arsitektur (final lock)

```text
system = execution + validation + recovery + governance
```

---

## Positioning

Ini **bukan** label generik “bot trading” semata; ini **governed execution engine** — eksekusi teratur dengan guard, validasi RPC, recovery berkelanjutan, dan exit adaptif sesuai blueprint ini.

---

*Dokumen ini mengunci spesifikasi perilaku dan batasan; implementasi harus selaras dengan invariant dan guard di atas.*
