package store

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"rlangga/internal/pumpws"
	"rlangga/internal/redisx"
)

const (
	keyTradesList = "trades:list"
	prefixDedupe  = "trade:dedupe:"
)

// Trade is one closed position (append-only log, PR-003).
type Trade struct {
	Mint        string  `json:"mint"`
	BotName     string  `json:"bot_name,omitempty"`
	BuySOL      float64 `json:"buy_sol"`
	SellSOL     float64 `json:"sell_sol"`
	PnLSOL      float64 `json:"pnl_sol"`
	Percent     float64 `json:"percent"`
	DurationSec int     `json:"duration_sec"`
	ExitReason  string  `json:"exit_reason,omitempty"` // adaptive exit: panic, grace_tp, …; recovery: recovery
	TS          int64   `json:"ts"`                    // unix detik — waktu tutup posisi / SELL terekam (alias analitik: sell)
	BuyTS       int64   `json:"buy_ts,omitempty"`      // unix detik — mulai monitor setelah BUY (buka posisi)
	// Snapshot dari payload WSS saat entry (opsional; untuk analisis SQL / tuning).
	EntryInitialBuy         float64 `json:"entry_initial_buy,omitempty"`    // initialBuy token (create)
	EntryMarketCapSOL       float64 `json:"entry_market_cap_sol,omitempty"` // proxy "ukuran" dari stream (bukan reserve on-chain)
	EntryPool               string  `json:"entry_pool,omitempty"`
	EntryPoolID             string  `json:"entry_pool_id,omitempty"`
	EntryStreamTimestampMs  int64   `json:"entry_stream_ts_ms,omitempty"` // timestamp ms dari field "timestamp" WSS jika ada
	EntryTxType             string  `json:"entry_tx_type,omitempty"`
	EntryPoolCreatedBy      string  `json:"entry_pool_created_by,omitempty"`
	EntryBurnedLiquidityPct float64 `json:"entry_burned_liquidity_pct,omitempty"`
	EntrySolInPool          float64 `json:"entry_sol_in_pool,omitempty"`
	EntryTokensInPool       float64 `json:"entry_tokens_in_pool,omitempty"`
}

// ApplyStreamEntryToTrade mengisi kolom entry_* dari payload WSS (snapshot pra-BUY).
func ApplyStreamEntryToTrade(tr *Trade, ev *pumpws.StreamEvent) {
	if tr == nil || ev == nil {
		return
	}
	if ev.HasInitialBuy {
		tr.EntryInitialBuy = ev.InitialBuy
	}
	if ev.HasMarketCapSOL {
		tr.EntryMarketCapSOL = ev.MarketCapSOL
	}
	tr.EntryPool = ev.Pool
	tr.EntryPoolID = ev.PoolID
	if ev.HasTimestamp {
		tr.EntryStreamTimestampMs = ev.TimestampMs
	}
	if ev.TxType != "" {
		tr.EntryTxType = ev.TxType
	}
	if ev.PoolCreatedBy != "" {
		tr.EntryPoolCreatedBy = ev.PoolCreatedBy
	}
	if ev.HasBurnedLiquidity {
		tr.EntryBurnedLiquidityPct = ev.BurnedLiquidityPct
	}
	if ev.HasSolInPool {
		tr.EntrySolInPool = ev.SolInPool
	}
	if ev.HasTokensInPool {
		tr.EntryTokensInPool = ev.TokensInPool
	}
}

// SaveTrade appends a trade to Redis (LPUSH). Duplicate identical payloads are ignored (SETNX on content hash).
// saved is false when dedupe skipped append (same payload hash as a prior save).
func SaveTrade(t Trade) (saved bool, err error) {
	if redisx.Client == nil {
		return false, fmt.Errorf("store: redis not initialized")
	}
	raw, err := json.Marshal(t)
	if err != nil {
		return false, err
	}
	sum := sha256.Sum256(raw)
	dedupeKey := prefixDedupe + fmt.Sprintf("%x", sum[:])

	ctx := context.Background()
	ok, err := redisx.Client.SetNX(ctx, dedupeKey, "1", 7*24*time.Hour).Result()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if err := redisx.Client.LPush(ctx, keyTradesList, string(raw)).Err(); err != nil {
		_ = redisx.Client.Del(ctx, dedupeKey).Err()
		return false, err
	}
	insertTradeSQLite(t)
	return true, nil
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
