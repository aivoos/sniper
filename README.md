# Sniper (RLANGGA)

Platform eksekusi terkendali (*governed execution engine*) untuk Solana: worker Go, Redis, validasi RPC, recovery berkelanjutan, guard risiko, dan dokumentasi PR berurutan. Ini **bukan** sekadar bot tanpa aturan — perilaku bisnis dikunci di dokumen.

**Status repositori:** spesifikasi dan kontrak lingkungan **siap implementasi**; kode aplikasi Go mengikuti roadmap PR (belum ada di repo ini).

---

## Dokumen utama

| Dokumen | Isi |
|---------|-----|
| [docs/rlangga-blueprint-v2.md](docs/rlangga-blueprint-v2.md) | Prinsip sistem, guard (waktu + kuota + loss), invariant |
| [docs/rlangga-env-contract.md](docs/rlangga-env-contract.md) | **Satu tabel** variabel `.env` |
| [docs/rlangga-full-stack.md](docs/rlangga-full-stack.md) | Infra → runtime → Redis → layanan eksternal |
| [docs/rlangga-production-hazards-and-fixes.md](docs/rlangga-production-hazards-and-fixes.md) | Race, edge case, perbaikan wajib |
| [docs/rlangga-repo-structure.md](docs/rlangga-repo-structure.md) | Layout modul Go yang direncanakan |

### Roadmap implementasi (PR)

| PR | Fokus |
|----|--------|
| [PR-001](docs/rlangga-pr-001-core-engine-recovery-validation.md) | Eksekusi, RPC, recovery, lock, idempotency |
| [PR-002](docs/rlangga-pr-002-adaptive-exit-pnl.md) | Quote, PnL, adaptive exit, monitor |
| [PR-003](docs/rlangga-pr-003-pnl-validation-analytics.md) | Trade log Redis, metrik, Telegram |
| [PR-004](docs/rlangga-pr-004-multi-bot.md) | Multi bot, orchestrator |
| [PR-005](docs/rlangga-pr-005-profit-guard.md) | Daily loss, kill switch, balance guard |

Mulai dari **PR-001**, lalu berurutan; integrasi antar PR dijelaskan di masing-masing dokumen.

---

## Stack (target)

- **Runtime:** Go 1.22+, Docker / Docker Compose  
- **Data:** Redis (lock, idempotency, trade log, stats)  
- **Eksekusi:** PumpPortal (+ fallback), RPC (mis. Helius)  
- **Observability:** Telegram (ringkasan / alert)

Detail: [docs/rlangga-full-stack.md](docs/rlangga-full-stack.md).

---

## Setelah kode ada (ringkas)

1. Salin dan isi variabel sesuai [docs/rlangga-env-contract.md](docs/rlangga-env-contract.md).  
2. `docker compose up --build` (atau jalankan `cmd/worker` sesuai [docs/rlangga-repo-structure.md](docs/rlangga-repo-structure.md)).  
3. Uji skenario di bagian *Definition of done* tiap PR.

---

## Lisensi

**Proprietary — private.** Lihat [LICENSE](LICENSE). Kode dan dokumentasi tidak dibuka untuk penggunaan publik; distribusi atau penyalinan tanpa izin tertulis dilarang.

Di GitHub: set repositori ke **Private** di *Settings → General → Danger zone* jika belum.

---

## Kontributor

Dokumentasi mengacu pada konvensi penamaan di [docs/rlangga-blueprint-v2.md](docs/rlangga-blueprint-v2.md). Perubahan perilaku bisnis harus selaras dengan blueprint dan `rlangga-env-contract.md`.
