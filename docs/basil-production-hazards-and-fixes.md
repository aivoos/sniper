# Hazard produksi: race, edge case, dan perbaikan wajib

**Jenjang dokumen:** referensi lintas-PR — melengkapi [basil-blueprint-v2.md](./basil-blueprint-v2.md) dan implementasi PR-001–PR-005.

Dokumen ini mencatat **12 kelas masalah** umum pada sistem eksekusi + recovery + guard. Perilaku yang salah sering muncul hanya di beban nyata, restart, atau RPC lambat.

---

## Ringkasan (indeks)

| # | Topik |
|---|--------|
| 1 | Race: BUY sukses vs *active set* |
| 2 | Double sell (monitor vs recovery) |
| 3 | Partial fill + loop retry |
| 4 | Quote basi → PnL salah |
| 5 | TTL lock kedaluwarsa terlalu cepat |
| 6 | Kuota di-increment di tempat salah |
| 7 | *Time window* melewati tengah malam |
| 8 | Recovery loop membebani RPC |
| 9 | Race indeks orchestrator multi-thread |
| 10 | Presisi float pada loss tracker |
| 11 | Sell sukses vs saldo RPC yang belum mutakhir |
| 12 | Recovery vs guard (konflik blokir) |

---

## 1. Race: BUY sukses vs penanda posisi aktif

**Masalah**

Urutan naif: BUY → sukses → `SetActivePosition()`. Jika proses **crash** di antara keduanya:

- On-chain sudah beli,
- State “posisi aktif” belum terset → rekonsiliasi bisa menganggap **orphan** atau sebaliknya salah klasifikasi, lalu perilaku SELL/recovery tidak konsisten.

**Perbaikan (wajib)**

**State dulu, baru aksi** — set penanda niat/posisi sebelum eksekusi BUY; bersihkan jika BUY gagal:

```go
// urutan mendekati atomik: state → action → rollback state jika gagal
SetActivePosition(mint)
success := BuyAndValidate(mint)

if !success {
    RemoveActivePosition(mint)
}
```

**Prinsip:** jangan mengandalkan “setelah BUY pasti sempat commit state”; gunakan urutan yang aman + rollback eksplisit.

---

## 2. Double sell: monitor vs recovery

**Masalah**

Goroutine **monitor** memicu SELL; bersamaan **recovery loop** juga memicu SELL pada mint yang sama → risiko transaksi ganda, error RPC, atau race saldo.

**Perbaikan**

Tambahkan status keluar di Redis (atau store setara):

```text
positions:state:<mint> = ACTIVE | EXITING
```

Logika:

```go
if state == EXITING {
    return // jangan double sell
}
```

Transisi ke `EXITING` harus **satu pemenang** (satu writer) — misalnya CAS atau lock ringan sebelum kirim SELL.

---

## 3. Partial fill dan loop retry (debu saldo)

**Masalah**

SELL sebagian sukses; saldo masih tersisa → retry. Quote dan slippage berubah; loop bisa **gagal terus** atau perilaku tidak terkendali.

**Perbaikan**

Abaikan sisa di bawah ambang debu:

```go
if balance < minDust {
    return // abaikan
}
```

**Definisi (contoh):** `MIN_DUST = 0.0001` SOL (atau setara token — sesuaikan dengan presisi jaringan).

---

## 4. Quote basi → keputusan exit salah

**Masalah**

Quote dipolling tiap 500 ms, tetapi jaringan/API lambat → PnL dihitung dari data **stale** → exit salah.

**Perbaikan**

- Catat *timestamp* quote; jika `quoteAge > 1s`, **lewati iterasi** monitor (atau jangan mengambil keputusan exit pada tick itu).

```go
if quoteAge > 1*time.Second {
    continue // skip iteration
}
```

---

## 5. TTL lock kedaluwarsa terlalu cepat

**Masalah**

Lock Redis TTL singkat (mis. 5 menit). Jika worker **hang** atau jaringan macet, TTL habis → mint bisa diambil proses lain → **double trade**.

**Perbaikan**

- Perpanjang TTL ke **10–15 menit**, **atau**
- **Refresh** lock secara periodik selama posisi masih aktif (heartbeat).

---

## 6. Kuota: increment di tempat salah

**Masalah**

Increment kuota pada **percobaan** BUY. Jika BUY gagal, kuota tetap terpakai → **under-trading** dan metrik salah.

**Perbaikan**

```go
if BuyAndValidate() {
    IncrementTradeCount()
}
```

Hanya increment setelah sukses (sesuai definisi bisnis blueprint).

---

## 7. *Time window* melewati tengah malam (contoh 20:00–02:00 WIB)

**Masalah**

Logika naif:

```text
if now >= 20 AND now <= 2  // salah untuk rentang yang memotong tengah malam
```

**Perbaikan**

Untuk rentang yang “membungkus” tengah malam (jam mulai lebih besar dari jam selesai pada satu hari kalender):

```text
if hour >= 20 || hour <= 2
```

Pastikan zona waktu **WIB** (atau yang dipilih) konsisten di server; uji unit untuk jam 00:30, 21:00, 19:59, 02:01.

---

## 8. Recovery loop membebani RPC

**Masalah**

Loop tiap 5 detik + **scan wallet** berat → saat scale atau rate limit RPC, permintaan gagal bersamaan.

**Perbaikan**

- Interval **10–15 detik**, **atau**
- **Adaptif:** lebih jarang saat idle, lebih sering saat ada anomaly (dengan batas atas).

---

## 9. Race indeks orchestrator (multi bot)

**Masalah**

`idx++` pada round-robin tanpa sinkronisasi → **data race** jika banyak goroutine.

**Perbaikan**

Gunakan penghitung atomik (atau mutex):

```go
atomic.AddInt32(&idx, 1) // modulo len(bots) saat membaca indeks
```

Pastikan modul `% len(bots)` dilakukan dengan indeks yang terbaca secara atomik.

---

## 10. Loss tracker dan float presisi

**Masalah**

`IncrByFloat` + akumulasi float → kesalahan pembulatan (`0.499999` vs `0.5`) memengaruhi kill switch.

**Perbaikan**

- **Bulatkan** ke N desimal (mis. **6**) sebelum bandingkan threshold atau sebelum tulis Redis; **atau**
- Simpan loss harian dalam **integer** unit terkecil (lamports) untuk akumulasi pasti.

---

## 11. SELL terkonfirmasi tetapi saldo RPC belum mutakhir

**Masalah**

RPC lag: tx sudah *confirmed*, saldo belum ter-update → logika mengira masih ada saldo → **retry SELL** tidak perlu atau berbahaya.

**Perbaikan**

Tunggu **1–2 detik** (dengan *backoff*) sebelum **cek saldo** pasca-SELL; atau andalkan konfirmasi tx + *parse* hasil, bukan hanya saldo akun seketika.

---

## 12. Recovery vs guard: konflik blokir

**Masalah**

Guard memblokir “semua” jalur ketika kuota habis atau kill switch — recovery **harus tetap bisa** menjual posisi lama.

**Perbaikan**

**Recovery mem-bypass guard** untuk path **SELL / likuidasi**. `CanTrade` hanya untuk **BUY / buka posisi baru** (selaras [PR-005](./basil-pr-005-profit-guard.md)).

---

## Ringkasan akhir

| # | Masalah | Arah perbaikan |
|---|---------|----------------|
| 1 | BUY vs active state | State → BUY → rollback jika gagal |
| 2 | Double sell | Status `EXITING` + satu penulis |
| 3 | Partial fill loop | `MIN_DUST`, henti retry debu |
| 4 | Quote stale | Lewati iterasi jika `quoteAge` besar |
| 5 | Lock TTL | 10–15 m atau refresh lock |
| 6 | Kuota | Increment hanya setelah BUY sukses |
| 7 | Midnight window | `||` untuk rentang memotong tengah malam |
| 8 | RPC overload | Interval recovery lebih besar / adaptif |
| 9 | Orchestrator `idx` | Atomik / mutex |
| 10 | Float loss | Bulatkan atau integer atom |
| 11 | Balance lag | Jeda sebelum cek saldo pasca-SELL |
| 12 | Recovery vs guard | SELL bypass guard |

---

*Implementasi konkret (nama fungsi, kunci Redis) harus selaras dengan PR terkait; dokumen ini mengunci **kelas bug** dan **pola perbaikan** agar review dan uji regresi tidak melewatkan edge case.*

*Nama env untuk `MIN_DUST`, `QUOTE_MAX_AGE_MS`, `LOCK_TTL_MIN`, dll.: [basil-env-contract.md](./basil-env-contract.md) §7.*
