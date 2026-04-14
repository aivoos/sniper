# Parity dengan .github/workflows/ci.yml — jalankan sebelum push: make ci
# Detail alur tes vs live: docs/testing-and-release.md

.PHONY: ci test test-race testv lint fmt vet wss-sample sim-engine sim-reset sim-session

# Samakan dengan COVERAGE_MIN di .github/workflows/ci.yml (satu sumber kebenaran angka).
COVERAGE_MIN ?= 71

fmt:
	go fmt ./...

vet:
	go vet ./...

# Tampilkan nama tiap tes (RUN / PASS / FAIL) di terminal — tanpa ambang coverage.
testv:
	go test ./internal/... -count=1 -v

# Tes + coverage tanpa -race (cocok jika belum ada gcc/CGO).
test:
	go test ./internal/... -count=1 -coverprofile=coverage.out -covermode=atomic
	@pct=$$(go tool cover -func=coverage.out | grep '^total:' | awk '{gsub(/%/,"",$$3); print $$3}'); \
	echo "total coverage: $${pct}% (min $(COVERAGE_MIN)% — set di Makefile / CI)"; \
	awk -v p="$$pct" -v m=$(COVERAGE_MIN) 'BEGIN { exit !(p+0 >= m+0) }' || { echo "coverage below $(COVERAGE_MIN)% — lihat docs/testing-and-release.md"; exit 1; }

# Sama seperti step "Test + coverage" di CI (dengan -race; perlu gcc).
test-race:
	CGO_ENABLED=1 go test ./internal/... -count=1 -race -coverprofile=coverage.out -covermode=atomic
	@pct=$$(go tool cover -func=coverage.out | grep '^total:' | awk '{gsub(/%/,"",$$3); print $$3}'); \
	echo "total coverage (race): $${pct}% (min $(COVERAGE_MIN)%)"; \
	awk -v p="$$pct" -v m=$(COVERAGE_MIN) 'BEGIN { exit !(p+0 >= m+0) }' || { echo "coverage below $(COVERAGE_MIN)%"; exit 1; }

# CI di GitHub memakai ubuntu (gcc tersedia) sehingga -race bisa dipakai.
# Lokal: `make test` tanpa race, atau `make test-race` jika gcc terpasang.
ci: fmt vet test
	@echo "OK — fmt+vet+test (coverage). CI juga menjalankan -race: gunakan 'make test-race' untuk parity penuh."

race:
	CGO_ENABLED=1 go test ./internal/... -count=1 -race -coverprofile=coverage.out -covermode=atomic

# Sampel frame JSON dari WebSocket (default PumpAPI). Muat .env jika ada. Lihat docs/wss-data-for-filters.md.
wss-sample:
	go run ./cmd/wss-sample -n 5

# Simulasi penuh: stream nyata + SIMULATE_ENGINE (tanpa tx on-chain). Set .env: SIMULATE_TRADING=0, SIMULATE_ENGINE=1, PUMP_WS_*.
sim-engine:
	go run ./cmd/worker

# Reset trade + dedupe + laporan + stat harian guard di Redis (sama REDIS_URL dengan worker). Lihat docs/sim-engine-tuning.md.
sim-reset:
	go run ./cmd/reset-pnl

# Sesi tuning bersih: reset lalu jalankan worker.
sim-session: sim-reset sim-engine
