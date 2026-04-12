# Parity lingkungan: lokal ↔ Git (CI) ↔ server

**Tujuan:** perilaku **1:1** — yang lulus di laptop sama dengan yang di CI dan yang jalan di server, supaya tidak ada “di saya jalan”.

---

## 1. Sumber kebenaran

| Lapisan | Sumber kebenaran |
|---------|------------------|
| Versi Go | `go.mod` (`go 1.22`) |
| Build & run kontainer | `Dockerfile` + `docker-compose.yml` di repo |
| Gate merge | GitHub Actions `.github/workflows/ci.yml` |
| Variabel runtime | [rlangga-env-contract.md](./rlangga-env-contract.md) |

Server produksi harus menjalankan **image yang sama** (build dari commit yang sama) atau setidaknya Dockerfile yang sama dengan yang diuji di CI.

---

## 2. Perintah yang harus sama

Di **lokal**, sebelum push, jalankan setidaknya:

```bash
make ci
```

Ini diselaraskan dengan job CI (lihat `Makefile`). Jika `make ci` gagal, PR seharusnya gagal juga.

---

## 3. GCC / `build-essential` (bukan orang atau akun luar)

**GCC** = kompiler C/C++ GNU — **bukan** nama user, **bukan** layanan pihak ketiga, **bukan** “jayceeliu” atau siapa pun.

Go membutuhkannya hanya jika:

- `CGO_ENABLED=1` **dan**
- Anda menjalankan `go test -race` (race detector memakai CGO di banyak platform).

**Parity praktis:**

| Lingkungan | `-race` |
|------------|---------|
| **GitHub Actions** (`ubuntu-latest`) | Biasanya **ada** `gcc` → `-race` jalan |
| **WSL/Ubuntu lokal tanpa gcc** | Pasang: `sudo apt install -y build-essential` **atau** selalu tes di dalam **Docker** (image `golang` sudah menyertakan toolchain) |
| **Server** (hanya jalankan **binary/container**) | Tidak perlu gcc di server jika Anda **tidak** mengompilasi di server; build di CI atau `docker build` |

Jadi: **gcc = toolchain standar di OS build**, bukan identitas atau dependency aneh.

---

## 4. Supaya lokal = CI (tanpa drama gcc)

1. Pasang **Go 1.22+** sama dengan `go.mod`.  
2. Opsi A: pasang **`build-essential`** (Ubuntu/WSL) lalu `make ci`.  
3. Opsi B: kerja hanya lewat **Docker** (`docker compose run …`) — image Go official sudah lengkap untuk build/test.  
4. Jangan mengandalkan env vars yang tidak ada di `.env.example` tanpa mendokumentasikannya.

---

## 5. Server nanti

- Deploy dari **artifact yang sama** dengan CI: image Docker hasil `docker build` pada commit/tag tertentu.  
- **Jangan** compile manual di server dengan versi Go berbeda dari `go.mod`.  
- Secret (RPC key, Telegram) hanya di env server / secret manager — tidak di git.

---

*Dokumen ini melengkapi [rlangga-test-standard.md](./rlangga-test-standard.md) soal “polisi” CI: aturan tertulis + lingkungan selaras.*
