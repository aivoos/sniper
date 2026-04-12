package store

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"rlangga/internal/redisx"
)

const (
	keyTradesList = "trades:list"
	prefixDedupe  = "trade:dedupe:"
)

// Trade is one closed position (append-only log, PR-003).
type Trade struct {
	Mint        string  `json:"mint"`
	BuySOL      float64 `json:"buy_sol"`
	SellSOL     float64 `json:"sell_sol"`
	PnLSOL      float64 `json:"pnl_sol"`
	Percent     float64 `json:"percent"`
	DurationSec int     `json:"duration_sec"`
	TS          int64   `json:"ts"`
}

// SaveTrade appends a trade to Redis (LPUSH). Duplicate identical payloads are ignored (SETNX on content hash).
func SaveTrade(t Trade) error {
	if redisx.Client == nil {
		return fmt.Errorf("store: redis not initialized")
	}
	raw, err := json.Marshal(t)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	dedupeKey := prefixDedupe + fmt.Sprintf("%x", sum[:])

	ctx := context.Background()
	ok, err := redisx.Client.SetNX(ctx, dedupeKey, "1", 7*24*time.Hour).Result()
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return redisx.Client.LPush(ctx, keyTradesList, string(raw)).Err()
}

// LoadRecent returns up to n newest trades (LPUSH order: index 0 = newest).
func LoadRecent(n int) ([]Trade, error) {
	if redisx.Client == nil {
		return nil, fmt.Errorf("store: redis not initialized")
	}
	if n <= 0 {
		return nil, nil
	}
	ctx := context.Background()
	vals, err := redisx.Client.LRange(ctx, keyTradesList, 0, int64(n-1)).Result()
	if err != nil {
		return nil, err
	}
	out := make([]Trade, 0, len(vals))
	for _, v := range vals {
		var t Trade
		if err := json.Unmarshal([]byte(v), &t); err != nil {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}
