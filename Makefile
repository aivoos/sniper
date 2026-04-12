# Parity dengan .github/workflows/ci.yml — jalankan sebelum push: make ci

.PHONY: ci test lint fmt vet

fmt:
	go fmt ./...

vet:
	go vet ./...

# Sama seperti CI: hanya ./internal/... (tanpa cmd/worker) + ambang coverage (PR-003+).
test:
	go test ./internal/... -count=1 -coverprofile=coverage.out -covermode=atomic
	@pct=$$(go tool cover -func=coverage.out | grep '^total:' | awk '{gsub(/%/,"",$$3); print $$3}'); \
	echo "total coverage: $${pct}% (min 89%)"; \
	awk -v p="$$pct" -v m=89 'BEGIN { exit !(p+0 >= m+0) }' || { echo "coverage below 89%"; exit 1; }

# CI di GitHub memakai ubuntu (gcc tersedia) sehingga -race bisa dipakai.
# Lokal tanpa gcc: jalankan `make test` saja, atau: apt install build-essential
ci: fmt vet test
	@echo "OK — mirror what CI runs (see also: go test -race if you have gcc)"

race:
	CGO_ENABLED=1 go test ./internal/... -count=1 -race -coverprofile=coverage.out -covermode=atomic
