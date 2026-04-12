# Sniper (RLANGGA)

[![CI](https://github.com/aivoos/sniper/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/aivoos/sniper/actions/workflows/ci.yml)

Platform eksekusi terkendali (*governed execution engine*) untuk Solana: worker Go, Redis, validasi RPC, recovery berkelanjutan, guard risiko, dan dokumentasi PR berurutan. Ini **bukan** sekadar bot tanpa aturan — perilaku bisnis dikunci di dokumen.

**Status implementasi:** **PR-001** + **PR-002** + **PR-003** (trade log Redis `trades:list`, agregasi, ringkasan Telegram opsional). **PR-004+** belum (multi-bot). Pump: `buy`/`sell`/`quote` memakai JSON dengan `signature`/`tx`/`sig` atau quote `sol`/`amount` — sesuaikan API nyata.

---

## Dokumen utama

| Dokumen | Isi |
|---------|-----|
| [docs/rlangga-blueprint-v2.md](docs/rlangga-blueprint-v2.md) | Prinsip sistem, guard (waktu + kuota + loss), invariant |
| [docs/rlangga-env-contract.md](docs/rlangga-env-contract.md) | **Satu tabel** variabel `.env` |
| [docs/rlangga-full-stack.md](docs/rlangga-full-stack.md) | Infra → runtime → Redis → layanan eksternal |
| [docs/rlangga-production-hazards-and-fixes.md](docs/rlangga-production-hazards-and-fixes.md) | Race, edge case, perbaikan wajib |
| [docs/rlangga-repo-structure.md](docs/rlangga-repo-structure.md) | Layout modul Go yang direncanakan |
| [docs/rlangga-test-standard.md](docs/rlangga-test-standard.md) | Standar tes: unit / simulasi / integrasi, CI, mocking |
| [docs/rlangga-dev-parity.md](docs/rlangga-dev-parity.md) | Parity lokal = CI = server; penjelasan GCC / `build-essential` |

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

## CI / gate merge

GitHub Actions (`.github/workflows/ci.yml`) menjalankan `gofmt`, `go vet`, `go test -race`, dan ambang coverage. Workflow **CI** dan **CD** bisa dijalankan manual dari tab *Actions* → pilih workflow → *Run workflow*. **Sebelum push**, samakan dengan CI: `make ci` (lihat [docs/rlangga-dev-parity.md](docs/rlangga-dev-parity.md)).

**Mengaktifkan Actions di repo:** *Settings* → *Actions* → *General* → *Actions permissions* → **Allow all actions and reusable workflows** (atau batasi sesuai kebijakan organisasi). Tanpa ini, workflow tidak jalan.

**CD (image):** `.github/workflows/cd.yml` membangun `Dockerfile` dan mendorong tag `latest` ke **GHCR** (`ghcr.io/<owner>/sniper`) pada push ke `main` yang mengubah kode Go/Dockerfile (atau lewat *workflow_dispatch*). Paket image muncul di *Packages* repositori; tarik dengan `docker pull ghcr.io/<owner>/sniper:latest` (login GHCR jika repo privat).

**GCC:** bukan nama orang atau layanan aneh — itu kompiler C standar; dipakai `go test -race` lewat CGO. Di Ubuntu/WSL tanpa gcc: pasang `build-essential` **atau** cukup `make test` tanpa `-race`; di CI Ubuntu sudah ada gcc.

**CI merah tanpa log / ~0 detik / tanpa job:** biasanya bukan gagal tes, tapi runner tidak jalan. Cek: (1) *Settings* → *Actions* → *General* → izin Actions; (2) akun: *Settings* → *Billing* → **Actions** — untuk repo **privat**, pastikan *Spending limit* tidak memblokir (repo publik umumnya gratis); (3) email GitHub sudah diverifikasi; (4) buka run di tab *Actions* — baca banner merah jika ada. Workflow **manual** hanya ada di branch yang sudah memuat `workflow_dispatch` (merge PR CI/CD ke `main` dulu).

---

## Menjalankan

```bash
# Lokal (butuh Redis, mis. docker run -d -p 6379:6379 redis:7)
export REDIS_URL=127.0.0.1:6379
export RPC_STUB=1
go run ./cmd/worker
```

```bash
# Docker Compose (set `RPC_STUB=1` di compose untuk dry-run tanpa pump/RPC nyata)
docker compose up --build
```

1. Salin [`.env.example`](.env.example) → `.env` dan isi sesuai [docs/rlangga-env-contract.md](docs/rlangga-env-contract.md).  
2. Uji skenario *Definition of done* di [PR-001](docs/rlangga-pr-001-core-engine-recovery-validation.md) lalu lanjut PR berikutnya.

---

## Lisensi

**Proprietary — private.** Lihat [LICENSE](LICENSE). Kode dan dokumentasi tidak dibuka untuk penggunaan publik; distribusi atau penyalinan tanpa izin tertulis dilarang.

Di GitHub: set repositori ke **Private** di *Settings → General → Danger zone* jika belum.

---

## Kontributor

Dokumentasi mengacu pada konvensi penamaan di [docs/rlangga-blueprint-v2.md](docs/rlangga-blueprint-v2.md). Perubahan perilaku bisnis harus selaras dengan blueprint dan `rlangga-env-contract.md`.
