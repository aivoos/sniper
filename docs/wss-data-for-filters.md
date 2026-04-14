# Data WebSocket untuk filter

**Rekomendasi operasional: satu sumber — [PumpAPI stream](https://pumpapi.io/) (`wss://stream.pumpapi.io`).** Payload-nya kaya (`txType`, `mint`, `marketCapSol`, `pool`, …) dan selaras dengan HTTP trade/quote kamu (`PUMPAPI_URL`). Menjalankan **PumpPortal + PumpAPI** paralel (primer + fallback) tetap didukung, tapi menggandakan sinyal dan mental overhead — hindari kecuali ada alasan tegas.

Dokumen di bawah menjelaskan **PumpAPI** dulu, lalu **PumpPortal** sebagai alternatif. Detail field bisa berubah — **uji dengan payload nyata** setelah connect.

**Catatan kode:** `internal/pumpws.ParseStreamEvent` mengisi `StreamEvent` dari JSON (utama untuk bentuk PumpAPI); gate `FILTER_WSS_*` di `internal/filter` (lihat [rlangga-env-contract.md](./rlangga-env-contract.md) §7c.2). Tanpa gate WSS, `ExtractMint` saja.

---

## 1. (Alternatif) PumpPortal — `wss://pumpportal.fun/api/data`

Sumber: [Real-time Updates | PumpPortal](https://pumpportal.fun/data-api/real-time), [PumpSwap Data API](https://pumpportal.fun/data-api/pump-swap).

### Subscribe (method yang dikirim setelah connect)

| Method | Isi sinyal |
|--------|------------|
| `subscribeNewToken` | Event **pembuatan token** (token baru di bonding curve). |
| `subscribeMigration` | Event **migrasi** token (biasanya keluar dari bonding → pool lain). |
| `subscribeTokenTrade` | **Trade** pada mint tertentu — perlu `keys`: array CA token. |
| `subscribeAccountTrade` | Trade oleh **akun** tertentu — perlu `keys`: array pubkey. |

Unsubscribe: `unsubscribeNewToken`, `unsubscribeTokenTrade`, `unsubscribeAccountTrade`, dll.

### Kebijakan koneksi

- **Satu koneksi WebSocket** per klien; tambah subscribe lewat pesan di koneksi yang sama (jangan buka banyak koneksi sekaligus — risiko timeout/ban sementara).
- **PumpSwap / data lanjutan:** dengan `?api-key=...` dan wallet terhubung ber-saldo minimal; biaya per pesan (lihat dokumen PumpSwap). Tanpa saldo cukup, stream bisa **dibatasi ke bonding curve saja**.

### Yang bisa dipakai untuk filter (konseptual)

- **Fase / konteks:** event dari `subscribeNewToken` vs `subscribeMigration` vs `subscribeTokenTrade` — beda makna (baru listing vs sudah migrasi vs aktivitas trade).
- **Trade per token / per akun:** setelah subscribe dengan `keys`, pesan menggambarkan aktivitas swap — berguna untuk **volume, frekuensi, arah buy/sell** (detail field ada di JSON respons; **cek sampel live**).
- **Bukan** dijamin berisi “likuiditas dalam SOL terkunci” sebagai satu angka agregat — seringkali perlu **kombinasi** dengan REST/RPC atau parser tx jika field di WS tidak cukup.

### Contoh JSON PumpPortal → hasil `ParseStreamEvent`

Bentuk pesan **bervariasi** per method; di bawah **ilustrasi** saja — bandingkan dengan log nyata.

| Contoh `msg` (potongan) | Yang terisi di `StreamEvent` |
|---------------------------|------------------------------|
| `{"mint":"So111…","signature":"abc…"}` | `Mint`, `Signature`; `TxType` kosong kecuali ada `txType` / `type` / `event`. |
| `{"method":"someEvent","mint":"So111…","solAmount":0.5}` | `Method` = `someevent`, `Mint`, `SolAmount`, `HasSolAmount`. |
| `{"params":{"mint":"So111…","type":"buy","solAmount":2}}` | `Mint`, `TxType` = `buy`, `SolAmount` (rekursif). |

**Penting:** `poolId`, `pool`, `marketCapSol`, `block`, `timestamp`, `initialBuy` diisi dari **seluruh pohon JSON** (root dulu, lalu nested seperti `params` / `result`) agar **primer + fallback WSS** memakai `FILTER_WSS_*` yang sama pada `StreamEvent` yang tersetandar.

---

## 2. PumpAPI (disarankan) — `wss://stream.pumpapi.io` ([pumpapi.io](https://pumpapi.io/))

Sumber: dokumentasi publik PumpAPI; endpoint umum **`wss://stream.pumpapi.io/`** — banyak klien **tanpa** payload subscribe tambahan; server mengirim aliran event **JSON** (agregat multi-venue).

**Bentuk nyata (contoh dari stream; field bisa berubah):** objek root berisi `signature`, `txType` (`create`, `buy`, `sell`, …), `mint`, `poolId`, `pool` (mis. `"pump"`), `txSigner`, `solAmount`, `tokenAmount` atau `initialBuy` (create), `marketCapSol`, `price`, `block`, `timestamp` (ms), metadata token pada create (`name`, `symbol`, `uri`, `supply`, `decimals`), dll. Parser di repo mengisi `StreamEvent` dari subset ini (lihat `internal/pumpws/stream_event.go`); field tambahan bisa ditambah ke parser jika filter kamu membutuhkannya.

Untuk pool AMM seperti **`pump-amm`**, beberapa payload juga menyediakan **reserve** seperti `solInPool`, `tokensInPool`, dan metadata risiko seperti `burnedLiquidity` serta `poolCreatedBy` — field ini bisa dipakai sebagai filter entry (lihat env `FILTER_MIN_ENTRY_SOL_IN_POOL`, `FILTER_MIN_BURNED_LIQUIDITY_PCT`, `FILTER_REJECT_POOL_CREATED_BY_CUSTOM`).

| Konsep | Kegunaan filter |
|--------|-----------------|
| `mint` | CA token — sama seperti yang dipakai worker. |
| `signature` | Idempotensi / dedupe event tx. |
| `txType` | **create** vs **buy** vs **sell** / lain — cocok untuk `FILTER_WSS_ALLOW_TX_TYPES`. |
| `solAmount` / `tokenAmount` / `initialBuy` | Ukuran arus (create sering punya `initialBuy`). |
| `marketCapSol`, `pool`, `poolId` | Saring bonding vs migrasi / venue. |
| `txSigner` | Filter by trader / bot. |
| `block`, `timestamp` | Urutan waktu / latensi. |

Beberapa sumber menjelaskan stream mencakup beberapa venue — **filter `pool`** saja sudah cukup untuk membatasi venue; **`txType`** (mis. hanya `create`) bersifat opsional jika kamu mau sinyal lebih sempit.

### Kebijakan

- Biasanya **satu koneksi per IP** (lihat FAQ penyedia); filter di sisi klien.

---

## 3. Perbandingan singkat

| Kebutuhan | Satu sumber PumpAPI | PumpPortal saja |
|-----------|---------------------|------------------|
| Setup | `PUMP_WS_URL=wss://stream.pumpapi.io`, fallback kosong | Subscribe JSON (`subscribeNewToken`, …) |
| Filter | `txType`, `pool`, `marketCapSol`, dll. dari payload | Per method subscribe + bentuk pesan berbeda |

Jika sudah pakai **PumpAPI untuk trade** (`PUMP_NATIVE` + `api.pumpapi.io`), **stream PumpAPI** adalah pasangan paling konsisten.

---

## 4. Implementasi di repo

| Komponen | Peran |
|----------|--------|
| `internal/pumpws/stream_event.go` | `ParseStreamEvent` — traversal JSON untuk mint, tipe tx, method, SOL / lamports, signature. |
| `internal/filter/wss.go` | `AllowStreamEvent` — gate jika `FILTER_WSS_*` di-set. |
| `internal/app/app.go` (`startPumpStream`) | Jika `FilterWSSGateActive()`, parse penuh + gate; jika tidak, `ExtractMint` saja. |

**Langkah operasional:** tetap **log sampel payload** dari primer/fallback di staging — field vendor bisa berubah; sesuaikan daftar kunci di `stream_event.go` jika perlu.

### Profil minimal: hanya `FILTER_WSS_POOL`

Cukup set **`FILTER_WSS_POOL`** ke satu atau beberapa venue, dipisah koma (case-insensitive). **Default disarankan: `pump-amm`** (pool PumpSwap AMM). Venue **`pump`** (bonding curve) hanya jika kamu sengaja memasukkannya — profil produksi yang konsisten dengan data analitik memakai **AMM saja**. Biarkan **`FILTER_WSS_ALLOW_TX_TYPES`**, **`FILTER_WSS_DENY_TX_TYPES`**, **`FILTER_WSS_MIN_MARKET_CAP_SOL`**, dll. **kosong atau 0** — gate WSS yang aktif hanya **pool**: payload harus punya field `pool` non-kosong dan cocok salah satu entri daftar. Lihat `internal/filter/wss.go` (`AllowStreamEvent`).

**Subscribe:** untuk sinyal “hanya token baru”, cukup `subscribeNewToken` (default PumpPortal); untuk aktivitas per-mint, `subscribeTokenTrade` + `keys` — lalu set `FILTER_WSS_ALLOW_TX_TYPES` / threshold SOL sesuai sampel nyata (opsional; tidak diperlukan untuk profil pool-only).

### Alat CLI `wss-sample` (frame JSON langsung)

Dari root repo:

```bash
make wss-sample
# setara: go run ./cmd/wss-sample -n 5
```

- Memuat **`.env`** jika ada (`PUMP_WS_URL`, `PUMP_WS_SUBSCRIBE_JSON`).
- Default URL: `PUMP_WS_URL`, atau **`wss://stream.pumpapi.io`** jika kosong.
- **PumpPortal:** jika `-url` mengandung `pumpportal` dan `-subscribe` kosong, mengirim `subscribeNewToken` + `subscribeMigration` (sama ide dengan worker).
- Flag: `-url`, `-n` (jumlah frame), `-timeout` (per frame), `-subscribe` (JSON array).

Contoh:

```bash
go run ./cmd/wss-sample -url 'wss://stream.pumpapi.io' -n 8
go run ./cmd/wss-sample -url 'wss://pumpportal.fun/api/data' -subscribe '[{"method":"subscribeNewToken"}]' -n 3
```

---

## 5. Rujukan kode

- `cmd/wss-sample/main.go`
- `internal/pumpws/stream_event.go`, `extract.go`
- `internal/filter/wss.go`
- `internal/app/app.go` (`startPumpStream`)

---

## 6. Rujukan cepat URL

| Endpoint | Dokumentasi |
|----------|-------------|
| PumpPortal real-time | https://pumpportal.fun/data-api/real-time |
| PumpPortal PumpSwap | https://pumpportal.fun/data-api/pump-swap |
| PumpAPI (HTTP/stream) | https://pumpapi.io/ |

*Tautan eksternal dapat berubah; utamakan halaman resmi terbaru.*
