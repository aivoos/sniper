# Sniper (BASIL)

Platform eksekusi terkendali (*governed execution engine*) untuk Solana: worker Go, Redis, validasi RPC, recovery berkelanjutan, guard risiko, dan dokumentasi PR berurutan. Ini **bukan** sekadar bot tanpa aturan — perilaku bisnis dikunci di dokumen.

**Status repositori:** spesifikasi dan kontrak lingkungan **siap implementasi**; kode aplikasi Go mengikuti roadmap PR (belum ada di repo ini).

---

## Dokumen utama

| Dokumen | Isi |
|---------|-----|
| [docs/basil-blueprint-v2.md](docs/basil-blueprint-v2.md) | Prinsip sistem, guard (waktu + kuota + loss), invariant |
| [docs/basil-env-contract.md](docs/basil-env-contract.md) | **Satu tabel** variabel `.env` |
| [docs/basil-full-stack.md](docs/basil-full-stack.md) | Infra → runtime → Redis → layanan eksternal |
| [docs/basil-production-hazards-and-fixes.md](docs/basil-production-hazards-and-fixes.md) | Race, edge case, perbaikan wajib |
| [docs/basil-repo-structure.md](docs/basil-repo-structure.md) | Layout modul Go yang direncanakan |

### Roadmap implementasi (PR)

| PR | Fokus |
|----|--------|
| [PR-001](docs/basil-pr-001-core-engine-recovery-validation.md) | Eksekusi, RPC, recovery, lock, idempotency |
| [PR-002](docs/basil-pr-002-adaptive-exit-pnl.md) | Quote, PnL, adaptive exit, monitor |
| [PR-003](docs/basil-pr-003-pnl-validation-analytics.md) | Trade log Redis, metrik, Telegram |
| [PR-004](docs/basil-pr-004-multi-bot.md) | Multi bot, orchestrator |
| [PR-005](docs/basil-pr-005-profit-guard.md) | Daily loss, kill switch, balance guard |

Mulai dari **PR-001**, lalu berurutan; integrasi antar PR dijelaskan di masing-masing dokumen.

---

## Stack (target)

- **Runtime:** Go 1.22+, Docker / Docker Compose  
- **Data:** Redis (lock, idempotency, trade log, stats)  
- **Eksekusi:** PumpPortal (+ fallback), RPC (mis. Helius)  
- **Observability:** Telegram (ringkasan / alert)

Detail: [docs/basil-full-stack.md](docs/basil-full-stack.md).

---

## Setelah kode ada (ringkas)

1. Salin dan isi variabel sesuai [docs/basil-env-contract.md](docs/basil-env-contract.md).  
2. `docker compose up --build` (atau jalankan `cmd/worker` sesuai [docs/basil-repo-structure.md](docs/basil-repo-structure.md)).  
3. Uji skenario di bagian *Definition of done* tiap PR.

---

## Lisensi

Belum ditetapkan — tambahkan berkas `LICENSE` saat tim memilih lisensi.

---

## Kontributor

Dokumentasi mengacu pada konvensi penamaan di [docs/basil-blueprint-v2.md](docs/basil-blueprint-v2.md). Perubahan perilaku bisnis harus selaras dengan blueprint dan `basil-env-contract.md`.
