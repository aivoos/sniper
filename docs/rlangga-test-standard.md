# RLANGGA test standard (Google-grade, locked)

**Jenjang dokumen:** referensi wajib — melengkapi [rlangga-blueprint-v2.md](./rlangga-blueprint-v2.md) §16 dan setiap PR. Nama file: `lower-kebab-case`.

**Catatan:** dokumen ini memakai nama **RLANGGA** (sesuai repositori); prinsip yang sama berlaku untuk seluruh sistem terkendali yang dijelaskan di blueprint.

---

## 0. Objektif

| Target | Keterangan |
|--------|------------|
| Tidak ada regresi | Perubahan kode tidak merusak perilaku yang sudah dikunci |
| Perilaku deterministik | Input yang sama → keputusan yang sama (di luar sumber acak eksternal) |
| Bug dapat direproduksi | Uji yang gagal memberi skenario minimal yang jelas |
| Deploy aman | CI sebagai *gate*: tes gagal → tidak merge / tidak deploy |

---

## 1. Piramida tes (wajib)

| Proporsi target | Jenis | Fokus |
|-----------------|-------|--------|
| **~70%** | Unit | Logika murni (exit, PnL, guard, waktu, kuota) |
| **~20%** | Simulasi | Kurva harga / urutan quote / skenario |
| **~10%** | Integrasi | Alur end-to-end dengan **mock** API/RPC |

**Aturan:** jangan membalik piramida (banyak integrasi lambat, sedikit unit).

---

## 2. Unit test (inti logika)

**Target cakupan modul kritis:** exit engine, perhitungan PnL, guard, jendela waktu, kuota.

Contoh (exit / *momentum drop* — sesuaikan nama fungsi dengan implementasi, mis. `ShouldSellAdaptive` di [PR-002](./rlangga-pr-002-adaptive-exit-pnl.md)):

```go
func TestExitMomentumDrop(t *testing.T) {

    state := &State{Peak: 10}

    pnl := 7.0
    elapsed := 6

    should := ShouldSell(pnl, elapsed, state)

    if !should {
        t.Fatal("expected sell on momentum drop")
    }
}
```

---

## 3. Tes deterministik (wajib)

Semua logika bisnis inti harus bisa dijalankan ulang dengan **hasil identik** (tanpa `time.Now()` tanpa injeksi jam, tanpa `rand` tanpa seed tetap).

```go
func TestDeterministicPnL(t *testing.T) {

    buy := 0.1
    sell := 0.11

    p1 := CalcPnL(buy, sell)
    p2 := CalcPnL(buy, sell)

    if p1 != p2 {
        t.Fatal("non deterministic pnl")
    }
}
```

---

## 4. Tes simulasi (kritis)

Simulasikan kurva *pump* atau urutan quote diskret.

```go
func TestPumpScenario(t *testing.T) {

    prices := []float64{
        0.1, 0.105, 0.11, 0.108, 0.103,
    }

    state := &State{}
    sold := false

    for i, p := range prices {

        pnl := CalcPnL(0.1, p)

        if ShouldSell(pnl, i, state) {
            sold = true
            break
        }
    }

    if !sold {
        t.Fatal("should have sold")
    }
}
```

---

## 5. Tes *replay* (lanjutan)

Input: urutan quote + *timestamp* (atau tick) simulasi dari log. Output: keputusan BUY/SELL harus **sama** dengan replay — mencegah keacakan tersembunyi dan memvalidasi regresi pada log produksi (tanpa data sensitif di repositori).

---

## 6. *Property test* (edge case)

| Properti | Contoh |
|----------|--------|
| PnL | Tidak boleh `NaN` / Inf |
| Saldo | Tidak negatif untuk invariant yang disepakati |
| Kuota | `remaining ≥ 0` setelah operasi valid |

Gunakan tabel tes atau *property-based* (opsional) untuk kombinasi batas.

---

## 7. Tes kegagalan

Contoh: SELL gagal berulang → harus retry sampai batas yang di dokumentasikan ([PR-001](./rlangga-pr-001-core-engine-recovery-validation.md)); tidak *silent fail*.

```go
func TestSellRetry(t *testing.T) {

    // mock: sell gagal 3x lalu sukses
    // assert: jumlah percobaan dan outcome akhir sesuai kontrak
}
```

---

## 8. Tes integrasi

| Alur | Catatan |
|------|---------|
| buy → validasi RPC → sell | Mock HTTP/RPC, bukan mainnet |
| recovery → jual orphan | Mock wallet + RPC |
| guard → blokir BUY | Mock Redis / config |

**Jangan** menguji langsung ke API produksi di CI.

---

## 9. Matriks skenario (wajib)

| Skenario | Keterangan |
|----------|------------|
| Fast pump | Kenaikan cepat lalu exit |
| Fake pump | Spike lalu dump — exit / momentum |
| Immediate dump | Stop loss / panic |
| Flat market | Timeout / max hold |
| Partial fill | Debu saldo, `MIN_DUST` ([hazards](./rlangga-production-hazards-and-fixes.md)) |
| API gagal | Fallback PumpAPI |
| RPC delay | Stale quote / retry ([hazards](./rlangga-production-hazards-and-fixes.md)) |

---

## 10. Target cakupan

| Ruang lingkup | Target |
|-----------------|--------|
| Logika bisnis (paket murni) | **Minimal ~80%** |
| Modul kritis (`exit`, `pnl`, `guard`, `aggregate`) | **90%+** disarankan |

Angka ini diukur per paket atau per repo setelah pemisahan jelas; sesuaikan dengan `go test -cover ./...`.

---

## 11. Larangan

| Larangan | Alasan |
|----------|--------|
| Logika inti tanpa tes | Regresi tidak terlihat |
| Perilaku acak di jalur keputusan | Tidak bisa diuji deterministik |
| Waktu nyata tanpa *mock* / injeksi `Clock` | Flaky test |
| Gagal diam-diam | Melanggar prinsip PR-001 / hazards |

---

## 12. CI gate (wajib)

Perintah minimal:

```bash
go test ./... -cover -count=1
```

**Gagal pipeline jika:**

- Ada tes yang gagal, atau  
- Cakupan di bawah ambang yang disepakati tim (mis. **di bawah 80%** pada paket yang ditandai *critical*).

Pengaturan ambang presisi (80% global vs per-paket) dilakukan dengan skrip `Makefile` atau CI (mis. `go-test-coverage` + threshold) — yang penting **aturan tertulis** dan konsisten.

---

## 13. Strategi mocking

| Lapisan | Mock |
|---------|------|
| RPC | Respons `getSignatureStatuses` / saldo tetap |
| Quote | Urutan harga dari fixture |
| Executor | HTTP transport dengan `httptest` atau interface |

Tujuan: isolasi unit dan integrasi tanpa jaringan produksi.

---

## 14. Frekuensi menjalankan tes

| Peristiwa | Wajib |
|-----------|--------|
| PR / merge request | `go test ./...` lulus |
| Sebelum deploy | Sama + gate CI |
| Refactor besar | Tes penuh + skenario regresi |

---

## 15. Insight

Tanpa tes yang mengikat logika ke perilaku yang diharapkan, sistem **bukan** verifikasi terkendali, melainkan **spekulasi** berisiko tinggi.

**Kunci:** **test = bagian dari sistem**, bukan tambahan setelah kode selesai.

---

## Kunci penguncian

| Prinsip | Makna |
|---------|--------|
| Final lock | Standar ini mengikuti praktik pengujian tingkat produk (acuan Google: piramida tes, determinisme, CI); detail per PR tetap di dokumen PR masing-masing. |

---

## Rujukan

- Blueprint §16: [rlangga-blueprint-v2.md](./rlangga-blueprint-v2.md)  
- Hazard produksi: [rlangga-production-hazards-and-fixes.md](./rlangga-production-hazards-and-fixes.md)  
- PR-002 (exit / PnL): [rlangga-pr-002-adaptive-exit-pnl.md](./rlangga-pr-002-adaptive-exit-pnl.md)
