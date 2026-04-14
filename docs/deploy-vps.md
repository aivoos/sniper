# Deploy worker di VPS (SSH + Docker)

Panduan ringkas untuk menjalankan bot di server Linux (Ubuntu/Debian) dengan SSH. Sebelum **uang nyata**: baca [go-live-checklist.md](./go-live-checklist.md) dan set `SIMULATE_ENGINE=0`, `SIMULATE_TRADING=0`, `RPC_STUB=0` hanya setelah siap.

---

## 1. Prasyarat

- VPS **Ubuntu 22.04/24.04** (atau Debian 12), akses **SSH** (kunci publik disarankan, non-root user + `sudo`).
- **Redis** — paling mudah lewat Docker Compose (service `redis` di repo).
- File **`.env`** lengkap di mesin lokal (jangan commit); salin ke VPS dengan aman (`scp` / secret manager).

---

## 2. SSH ke VPS

```bash
ssh -i ~/.ssh/kunci_anda.pem user@IP_VPS
# atau
ssh user@hostname
```

Pertama kali: update sistem, buat user non-root jika perlu, pasang firewall.

```bash
sudo apt update && sudo apt upgrade -y
```

---

## 3. Pasang Docker + Compose

```bash
sudo apt install -y ca-certificates curl
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo usermod -aG docker "$USER"
# logout + login agar grup docker aktif
```

Verifikasi: `docker compose version`

---

## 4. Klon repo di VPS

```bash
cd ~
git clone https://github.com/ORG/sniper.git
cd sniper
git checkout main   # atau branch deploy Anda
```

---

## 5. Salin `.env` dari laptop (contoh)

Di **mesin lokal** (bukan di VPS):

```bash
scp -i ~/.ssh/kunci_anda.pem /path/ke/.env user@IP_VPS:~/sniper/.env
```

Atau buat `.env` langsung di VPS dengan `nano ~/sniper/.env` (minimal: `REDIS_URL`, RPC, Pump, kunci — lihat [rlangga-env-contract.md](./rlangga-env-contract.md)).

---

## 6. Compose: Redis + worker

File [docker-compose.yml](../docker-compose.yml) punya `redis` + `worker` dengan env minimal. Untuk produksi, **inject seluruh variabel** dari `.env`:

Buat **`docker-compose.override.yml`** di folder repo (tidak wajib di-commit; bisa `.gitignore`):

```yaml
services:
  worker:
    env_file:
      - .env
    environment:
      REDIS_URL: redis:6379
```

`REDIS_URL` di dalam jaringan Docker harus **`redis:6379`** (nama service), bukan `127.0.0.1`.

Build dan jalankan:

```bash
cd ~/sniper
docker compose build
docker compose up -d
```

Cek log:

```bash
docker compose logs -f worker
```

---

## 7. Health check

Worker membuka **`/health`** di port **8080** (lihat `HEALTH_PORT` di `.env`). Dari VPS:

```bash
curl -s http://127.0.0.1:8080/health
```

Di `Dockerfile` sudah ada `HEALTHCHECK` ke endpoint ini. Untuk monitoring luar, buka port 8080 di firewall **hanya** jika perlu (atau pakai reverse proxy + auth).

---

## 8. Firewall (ufw) — contoh

```bash
sudo ufw allow OpenSSH
sudo ufw allow 22/tcp
# opsional: jangan buka 8080 ke publik kecuali perlu
sudo ufw enable
sudo ufw status
```

---

## 9. Restart setelah ubah `.env`

```bash
cd ~/sniper
docker compose up -d --build worker
# atau tanpa rebuild image:
docker compose up -d worker
```

---

## 10. Tanpa Docker (binary saja)

Alternatif: pasang **Go 1.22+** dan **Redis** (`apt install redis-server` atau Redis cloud), build:

```bash
cd ~/sniper
go build -o sniper-worker ./cmd/worker
export $(grep -v '^#' .env | xargs)   # hati-hati spasi/kutip di .env
./sniper-worker
```

Untuk 24/7 gunakan **systemd** (unit file `ExecStart=/home/user/sniper/sniper-worker`, `WorkingDirectory=...`, `EnvironmentFile=/home/user/sniper/.env`).

---

## 11. Checklist singkat produksi

| Item | Catatan |
|------|---------|
| `.env` | Tidak di Git; permission `chmod 600 .env` |
| `REDIS_URL` | Persistensi Redis (AOF) jika perlu data awet |
| Live trading | `SIMULATE_ENGINE=0`, `RPC_STUB=0`, kunci & RPC mainnet |
| Telegram | `TELEGRAM_*` terisi; bot sudah `/start` |
| WSS | Satu proses worker — hindari duplikat (policy violation 1008) |

Detail: [go-live-checklist.md](./go-live-checklist.md).
