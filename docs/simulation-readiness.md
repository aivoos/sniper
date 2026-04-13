# Laporan kesiapan: simulasi “real” (stream + paper engine)

**Simulasi real** di konteks repo ini berarti: **stream data live** (mis. Pump WebSocket) + **tidak ada transaksi swap nyata** untuk BUY/SELL, dengan **`SIMULATE_ENGINE=1`** (alur BUY → monitor → SELL dicatat dari tick/quote sintetis). Ini **bukan** go-live mainnet dengan uang penuh — untuk itu ikuti [go-live-checklist.md](./go-live-checklist.md).

---

## Status teknis (per verifikasi CI lokal)

| Item | Status |
|------|--------|
| `make ci` (fmt, vet, tes + ambang coverage) | Lulus (total coverage ~**77,8%**, minimum **76%**) |
| `make test-race` | Lulus (parity dengan step `-race` di GitHub Actions) |
| Tes tambahan terbaru | `RPC_URLS` invalid / kosong; `ResetReportState`; guard `config.C == nil` di `HandleMint` setelah BUY |

**Kesimpulan singkat:** dari sisi **gate kode + tes otomatis**, sistem **siap** dijalankan untuk **simulasi end-to-end** dengan konfigurasi yang tepat (Redis, WS, `SIMULATE_ENGINE`, RPC sesuai skenario).

---

## Checklist environment sebelum menjalankan worker simulasi

1. **`REDIS_URL`** — wajib untuk dedupe, trade log, guard, laporan.
2. **`PUMP_WS_*` / sumber stream** — sesuai kontrak di [rlangga-env-contract.md](./rlangga-env-contract.md); tanpa stream, tidak ada sinyal “real”.
3. **`SIMULATE_ENGINE=1`** — engine paper (stub eksekusi + quote sintetis yang bisa di-tune lewat `SIMULATE_SYNTH_*`).
4. **`RPC_STUB`** — untuk uji RPC tanpa jaringan: `1`; untuk verifikasi failover / status tx: `0` dan set **`RPC_URL`** atau **`RPC_URLS`** (koma) ke endpoint valid.
5. **Reset state paper (opsional)** — `go run ./cmd/reset-pnl` atau setara (Redis trades + counter laporan + guard harus konsisten dengan dokumentasi perintah tersebut).

---

## Yang belum wajib untuk simulasi, tapi wajib untuk live uang nyata

- Wallet dengan saldo, **`RPC_URL`/`RPC_URLS`** stabil, Pump API/gateway production, supervisor, alerting, dan prosedur di **go-live checklist**.

---

## Rujukan

- Urutan tes → staging → produksi: [testing-and-release.md](./testing-and-release.md).
- Kontrak variabel `.env`: [rlangga-env-contract.md](./rlangga-env-contract.md).
- Perbedaan implementasi vs dokumen PR/blueprint (wallet stub, recovery): [implementation-vs-spec.md](./implementation-vs-spec.md).
