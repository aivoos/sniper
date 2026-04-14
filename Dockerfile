FROM golang:1.22-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker


FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata curl && adduser -D -H -u 10001 app

WORKDIR /app
COPY --from=builder /out/worker /app/worker

USER app

ENV GODEBUG=madvdontneed=1

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1

CMD ["/app/worker"]
