# Parity dengan .github/workflows/ci.yml — jalankan sebelum push: make ci

.PHONY: ci test lint fmt vet

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./... -count=1 -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out | grep '^total:'

# CI di GitHub memakai ubuntu (gcc tersedia) sehingga -race bisa dipakai.
# Lokal tanpa gcc: jalankan `make test` saja, atau: apt install build-essential
ci: fmt vet test
	@echo "OK — mirror what CI runs (see also: go test -race if you have gcc)"

race:
	CGO_ENABLED=1 go test ./... -count=1 -race -coverprofile=coverage.out -covermode=atomic
