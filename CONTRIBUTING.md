# Kontribusi

## Alur pull request

1. Buat branch dari `main` yang sudah di-*pull*:
   ```bash
   git checkout main && git pull origin main
   git checkout -b feat/nama-fitur
   ```
2. Ubah kode / dokumen; jalankan gate yang sama dengan CI:
   ```bash
   make ci
   ```
   (Lihat [docs/rlangga-dev-parity.md](docs/rlangga-dev-parity.md) jika `make` atau `gcc` belum tersedia.)
3. Commit dan push:
   ```bash
   git push -u origin feat/nama-fitur
   ```
4. Buka PR ke `main` di GitHub — template akan muncul otomatis (`.github/pull_request_template.md`).

## Roadmap fitur

Urutan referensi: PR-001 → PR-005 — lihat [README.md](README.md) dan `docs/rlangga-pr-*.md`.
