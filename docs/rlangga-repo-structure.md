# Struktur repositori RLANGGA (final)

**Jenjang dokumen:** level 2 — turunan dari [rlangga-blueprint-v2.md](./rlangga-blueprint-v2.md)  
**Status:** baseline produksi (kerangka modular)

Dokumen ini memetakan layout kode Go (`module rlangga`), titik masuk worker, paket `internal`, kontainer, dan variabel lingkungan. Spesifikasi perilaku bisnis (guard, kuota, jendela waktu) tetap mengacu pada blueprint induk. Arsitektur lapisan penuh (infra → data → observability): [rlangga-full-stack.md](./rlangga-full-stack.md). Hazard produksi (race, edge case): [rlangga-production-hazards-and-fixes.md](./rlangga-production-hazards-and-fixes.md). Kontrak variabel lingkungan: [rlangga-env-contract.md](./rlangga-env-contract.md). Standar pengujian: [rlangga-test-standard.md](./rlangga-test-standard.md).

**Peringatan:** cuplikan kode Go di bagian bawah dokumen ini bersifat **historis / pedagogis**. Perilaku aktual ada di file sumber; penyimpangan vs desain dicatat di [implementation-vs-spec.md](./implementation-vs-spec.md).

---

## Pohon direktori

```text
rlangga/
├── cmd/
│   ├── worker/
│   │   └── main.go
│   └── reset-pnl/
│       └── main.go
│
├── internal/
│   ├── app/
│   ├── bot/
│   ├── executor/
│   ├── rpc/
│   ├── recovery/
│   ├── monitor/
│   ├── exit/
│   ├── pnl/
│   ├── quote/
│   ├── store/
│   ├── aggregate/
│   ├── report/
│   ├── guard/
│   ├── orchestrator/
│   ├── lock/
│   ├── idempotency/
│   ├── wallet/
│   ├── redisx/
│   ├── pumpws/
│   ├── pumpnative/
│   ├── sellguard/
│   ├── testutil/
│   └── log/
│
├── go.mod
├── Dockerfile
├── docker-compose.yml
└── .env
```

---

## `go.mod`

```go
module rlangga

go 1.22

require github.com/redis/go-redis/v9 v9.0.0
```

---

## `cmd/worker/main.go`

Titik masuk: inisialisasi aplikasi, recovery saat startup, loop recovery berkelanjutan, lalu worker.

```go
package main

import (
    "rlangga/internal/app"
    "rlangga/internal/recovery"
)

func main() {
    app.Init()

    // startup recovery
    recovery.RecoverAll()

    // continuous recovery
    go recovery.StartLoop()

    app.StartWorker()
}
```

---

## `internal/app/app.go`

Bootstrap dan loop worker (placeholder listener / `HandleMint`).

```go
package app

import "fmt"

func Init() {
    fmt.Println("RLANGGA INIT")
}

func StartWorker() {
    fmt.Println("Worker running...")
    // TODO: listener → HandleMint()
}
```

---

## `internal/executor/executor.go`

Eksekusi buy dengan validasi RPC; sell aman dengan retry.

```go
package executor

import (
    "time"
    "rlangga/internal/rpc"
)

func BuyAndValidate(mint string) bool {
    sig := "tx-buy" // TODO real API
    return rpc.WaitTxConfirmed(sig)
}

func SafeSellWithValidation(mint string) {
    for i := 0; i < 5; i++ {
        sig := "tx-sell"

        if rpc.WaitTxConfirmed(sig) {
            return
        }

        time.Sleep(500 * time.Millisecond)
    }
}
```

---

## `internal/recovery/recovery.go`

Scan wallet lewat RPC; jual token dengan saldo lebih dari nol; loop periodik.

```go
package recovery

import (
    "time"
    "rlangga/internal/rpc"
    "rlangga/internal/executor"
)

func RecoverAll() {
    tokens := rpc.GetWalletTokens()

    for _, t := range tokens {
        if t.Amount > 0 {
            executor.SafeSellWithValidation(t.Mint)
        }
    }
}

func StartLoop() {
    for {
        RecoverAll()
        time.Sleep(10 * time.Second)
    }
}
```

---

## `internal/rpc/rpc.go`

Abstraksi RPC: konfirmasi transaksi, daftar token wallet.

```go
package rpc

import "time"

type Token struct {
    Mint   string
    Amount float64
}

func WaitTxConfirmed(sig string) bool {
    for i := 0; i < 10; i++ {
        return true
        time.Sleep(300 * time.Millisecond)
    }
    return false
}

func GetWalletTokens() []Token {
    return []Token{}
}
```

---

## `internal/pnl/pnl.go`

Perhitungan PnL persentase dari harga buy/sell.

```go
package pnl

func CalcPnL(buy, sell float64) float64 {
    return (sell - buy) / buy * 100
}
```

---

## `internal/quote/quote.go`

Quote jual (placeholder — ganti API nyata).

```go
package quote

func GetSellQuote(mint string) float64 {
    return 0.11 // TODO real API
}
```

---

## `internal/exit/exit.go`

Mesin exit adaptif: panic SL, SL, TP, trailing momentum, max hold.

```go
package exit

type State struct {
    Peak float64
}

func ShouldSell(pnl float64, elapsed int, s *State) bool {

    if pnl > s.Peak {
        s.Peak = pnl
    }

    if pnl <= -8 {
        return true
    }

    if pnl <= -5 {
        return true
    }

    if pnl >= 7 && elapsed >= 5 {
        return true
    }

    if (s.Peak - pnl) >= 2.5 {
        return true
    }

    if elapsed >= 15 {
        return true
    }

    return false
}
```

---

## `internal/monitor/monitor.go`

Loop pemantauan posisi: quote → PnL → keputusan exit → sell aman.

```go
package monitor

import (
    "time"
    "rlangga/internal/quote"
    "rlangga/internal/pnl"
    "rlangga/internal/exit"
    "rlangga/internal/executor"
)

func Monitor(mint string, buy float64) {

    state := &exit.State{}
    start := time.Now()

    for {
        elapsed := int(time.Since(start).Seconds())

        q := quote.GetSellQuote(mint)
        p := pnl.CalcPnL(buy, q)

        if exit.ShouldSell(p, elapsed, state) {
            executor.SafeSellWithValidation(mint)
            return
        }

        time.Sleep(500 * time.Millisecond)
    }
}
```

---

## `internal/store/store.go`

Model trade untuk persistensi / log (isi minimal).

```go
package store

type Trade struct {
    Mint string
    PnL  float64
}
```

---

## `internal/aggregate/aggregate.go`

Agregasi metrik (placeholder).

```go
package aggregate

func Compute() {}
```

---

## `internal/report/report.go`

Saluran laporan (misalnya Telegram).

```go
package report

func Send(msg string) {}
```

---

## `internal/guard/guard.go`

Gate trading (placeholder — hubungkan ke blueprint: waktu, kuota, loss).

```go
package guard

func CanTrade() bool {
    return true
}
```

---

## `internal/orchestrator/orchestrator.go`

Orkestrasi multi-bot (placeholder).

```go
package orchestrator

func NextBot() int {
    return 0
}
```

---

## `internal/lock/lock.go`

Kunci per mint / operasi.

```go
package lock

func Lock(mint string) bool {
    return true
}
```

---

## `internal/idempotency/idempotency.go`

Cegah duplikasi aksi pada mint yang sama.

```go
package idempotency

func IsDuplicate(mint string) bool {
    return false
}
```

---

## `internal/wallet/wallet.go`

Saldo / wallet helper.

```go
package wallet

func GetBalance() float64 {
    return 1.0
}
```

---

## `internal/log/log.go`

Logging sederhana (ganti dengan `slog` / zerolog sesuai kebutuhan).

```go
package log

import "fmt"

func Info(msg string) {
    fmt.Println(msg)
}
```

---

## `Dockerfile`

```dockerfile
FROM golang:1.22

WORKDIR /app

COPY . .

RUN go build -o worker ./cmd/worker

CMD ["./worker"]
```

---

## `docker-compose.yml`

Worker + Redis.

```yaml
version: "3"

services:
  redis:
    image: redis:7
    restart: always

  worker:
    build: .
    restart: always
    depends_on:
      - redis
```

---

## `.env`

Daftar lengkap nama variabel dan makna: [rlangga-env-contract.md](./rlangga-env-contract.md). Contoh minimal:

```env
REDIS_URL=redis:6379
RPC_URL=xxx
PUMPPORTAL_URL=xxx
PUMPAPI_URL=xxx
```

---

## Menjalankan

```bash
docker-compose up --build -d
```

---

## Status baseline

| Aspek | Keterangan |
|--------|------------|
| Compile | Ya |
| Modular | Ya (`internal/` per domain) |
| Extensible | Ya (hook tes + mode stub/simulasi) |
| Production baseline | Sebagian — integrasi **saldo SOL** dan **scan token wallet** untuk recovery masih stub; lihat [implementation-vs-spec.md](./implementation-vs-spec.md) |

---

## Catatan implementasi

- **`rpc.WaitTxConfirmed`:** memanggil JSON-RPC `getSignatureStatuses` (bukan return konstan); failover antar-URL di `RPC_URLS`. `RPC_STUB=1` mempertahankan perilaku uji tanpa jaringan.
- **`internal/log`:** nama paket `log` bentrok dengan standar library `log` jika diimpor bersamaan; pertimbangkan rename paket ke `logger` atau `applog` saat integrasi penuh.
- **Blueprint:** guard waktu, kuota harian, daily loss, dan reporting Telegram dijabarkan di [rlangga-blueprint-v2.md](./rlangga-blueprint-v2.md); modul `guard` / `report` terhubung ke `.env`.
- **Cuplikan kode di bawah** (Init placeholder, `TODO real API`, dll.) **tidak** menggantikan pembacaan sumber aktual — lihat peringatan di atas.
