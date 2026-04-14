package pumpws

import (
	"encoding/json"
	"strconv"
	"strings"
)

// StreamEvent ringkasan field dari payload WSS untuk filter pra-BUY.
// Stream PumpAPI (pumpapi.io) sering mengirim objek root dengan txType, mint, poolId, pool, marketCapSol,
// solAmount, block, timestamp, txSigner, dll. PumpPortal bisa berbentuk berbeda — lihat docs/wss-data-for-filters.md.
// Field kosong berarti tidak ditemukan di JSON.
type StreamEvent struct {
	Mint           string
	TxType         string // dinormalisasi lowercase (perbandingan allow/deny)
	Method         string // mis. echo subscribe / channel
	Signature      string
	SolAmount      float64 // SOL (atau dikonversi dari lamports jika hanya itu yang ada)
	HasSolAmount   bool
	TokenAmount    float64
	HasTokenAmount bool
	// Field umum pada event PumpAPI / trade (root atau nested params/result — lihat fillTradeFromTree)
	PoolID          string
	Pool            string // dinormalisasi lowercase
	TxSigner        string
	MarketCapSOL    float64
	HasMarketCapSOL bool
	// Pool reserves snapshot (khusus AMM/DEX stream seperti pump-amm).
	TokensInPool    float64
	HasTokensInPool bool
	SolInPool       float64
	HasSolInPool    bool
	// Pool metadata (berguna untuk filter risiko).
	PoolCreatedBy  string // mis. pump (migration) atau custom (manual) — apa adanya dari payload
	PoolFeeRate    float64
	HasPoolFeeRate bool
	// BurnedLiquidityPct: persentase LP yang dibakar (0-100). -1 bila tidak tersedia/invalid.
	BurnedLiquidityPct float64
	HasBurnedLiquidity bool
	// Token safety (WSS-derived, no RPC).
	TokenProgram string   // spl-token atau spl-token-2022
	Extensions   []string // key names inside tokenExtensions map (lowercase)
	// MintAuthority / FreezeAuthority: true when authority is present (non-nil) in payload.
	HasMintAuthority   bool
	HasFreezeAuthority bool
	// InitialBuy (field JSON "initialBuy" pada event create PumpAPI): jumlah token pada pembelian awal creator — dipakai filter/tuning; terpisah dari TokenAmount agar filter tidak bergantung pada trade buy lain.
	InitialBuy    float64
	HasInitialBuy bool
	Block         uint64
	HasBlock      bool
	TimestampMs   int64 // unix ms dari field "timestamp" jika ada
	HasTimestamp  bool
}

var (
	// Jangan sertakan "name" — bentrok dengan nama token (mis. event create di PumpAPI).
	txTypeKeys = []string{"txType", "type", "event", "tx_type", "action", "kind", "op"}
	methodKeys = []string{"method", "channel", "topic"}
	sigKeys    = []string{"signature", "sig", "txSig", "transactionSignature"}
	solKeys    = []string{"solAmount", "sol_amount", "sol", "nativeSol", "native_sol", "amountSOL", "amount_sol"}
	tokenKeys  = []string{"tokenAmount", "token_amount", "amount", "qty", "uiAmount", "ui_amount"}
	lamportsK  = []string{"lamports"}
)

// ParseStreamEvent mem-parsing satu frame JSON WebSocket menjadi StreamEvent.
// Mengembalikan ok=false jika bukan JSON valid.
func ParseStreamEvent(msg []byte) (ev StreamEvent, ok bool) {
	var raw interface{}
	if decodeJSONRoot(msg, &raw) != nil {
		return ev, false
	}
	ev.Mint = walkForMint(raw)
	ev.TxType = strings.ToLower(strings.TrimSpace(findFirstString(raw, txTypeKeys)))
	ev.Method = strings.ToLower(strings.TrimSpace(findFirstString(raw, methodKeys)))
	ev.Signature = strings.TrimSpace(findFirstString(raw, sigKeys))

	if f, has := findFirstFloat(raw, solKeys); has {
		ev.SolAmount = f
		ev.HasSolAmount = true
	} else if lam, hasL := findFirstInt64ish(raw, lamportsK); hasL {
		ev.SolAmount = float64(lam) / 1e9
		ev.HasSolAmount = true
	}

	if f, has := findFirstFloat(raw, tokenKeys); has {
		ev.TokenAmount = f
		ev.HasTokenAmount = true
	}
	// Trade fields: isi dari root dulu, lalu nested (params/result/…) — dua sumber WSS memakai gate/filter yang sama.
	fillTradeFromTree(raw, &ev)
	return ev, true
}

// fillTradeFromTree mengunjungi setiap objek JSON; fillTradeFromMap hanya mengisi field yang masih kosong
// (nilai di root / objek dangkal menang atas nested lebih dalam bila keduanya ada).
func fillTradeFromTree(v interface{}, ev *StreamEvent) {
	switch t := v.(type) {
	case map[string]interface{}:
		fillTradeFromMap(t, ev)
		for _, val := range t {
			fillTradeFromTree(val, ev)
		}
	case []interface{}:
		for _, el := range t {
			fillTradeFromTree(el, ev)
		}
	}
}

func fillTradeFromMap(m map[string]interface{}, ev *StreamEvent) {
	if ev.PoolID == "" {
		if s := topString(m, "poolId"); s != "" {
			ev.PoolID = s
		}
	}
	if ev.Pool == "" {
		if s := topString(m, "pool"); s != "" {
			ev.Pool = strings.ToLower(s)
		}
	}
	if ev.TxSigner == "" {
		if s := topString(m, "txSigner"); s != "" {
			ev.TxSigner = s
		}
	}
	if !ev.HasMarketCapSOL {
		if v, ok := m["marketCapSol"]; ok {
			if f, ok := toFloat64(v); ok {
				ev.MarketCapSOL = f
				ev.HasMarketCapSOL = true
			}
		}
	}
	if !ev.HasTokensInPool {
		if v, ok := m["tokensInPool"]; ok {
			if f, ok := toFloat64(v); ok {
				ev.TokensInPool = f
				ev.HasTokensInPool = true
			}
		}
	}
	if !ev.HasSolInPool {
		if v, ok := m["solInPool"]; ok {
			if f, ok := toFloat64(v); ok {
				ev.SolInPool = f
				ev.HasSolInPool = true
			}
		}
	}
	if ev.PoolCreatedBy == "" {
		if s := topString(m, "poolCreatedBy"); s != "" {
			ev.PoolCreatedBy = strings.ToLower(strings.TrimSpace(s))
		}
	}
	if !ev.HasPoolFeeRate {
		if v, ok := m["poolFeeRate"]; ok {
			if f, ok := toFloat64(v); ok {
				ev.PoolFeeRate = f
				ev.HasPoolFeeRate = true
			}
		}
	}
	if !ev.HasBurnedLiquidity {
		// burnedLiquidity biasanya "100%" atau "99%". Simpan sebagai angka 0-100.
		if s := topString(m, "burnedLiquidity"); s != "" {
			ev.BurnedLiquidityPct, ev.HasBurnedLiquidity = parsePctString(s)
		}
	}
	if ev.TokenProgram == "" {
		if s := topString(m, "tokenProgram"); s != "" {
			ev.TokenProgram = strings.ToLower(strings.TrimSpace(s))
		}
	}
	// tokenExtensions is usually an object map: { "transferFeeConfig": {...}, ... }
	if len(ev.Extensions) == 0 {
		if v, ok := m["tokenExtensions"]; ok {
			if mm, ok := v.(map[string]interface{}); ok && len(mm) > 0 {
				out := make([]string, 0, len(mm))
				for k := range mm {
					kk := strings.ToLower(strings.TrimSpace(k))
					if kk != "" {
						out = append(out, kk)
					}
				}
				if len(out) > 0 {
					ev.Extensions = out
				}
			}
		}
	}
	// Authorities: payloads use null/None when disabled.
	// We treat any non-nil value (string/object) as "present".
	if !ev.HasMintAuthority {
		if v, ok := m["mintAuthority"]; ok && v != nil {
			// Some vendors might send "None" as string; normalize it to absent.
			if s, ok := v.(string); ok && strings.EqualFold(strings.TrimSpace(s), "none") {
				// absent
			} else {
				ev.HasMintAuthority = true
			}
		}
	}
	if !ev.HasFreezeAuthority {
		if v, ok := m["freezeAuthority"]; ok && v != nil {
			if s, ok := v.(string); ok && strings.EqualFold(strings.TrimSpace(s), "none") {
				// absent
			} else {
				ev.HasFreezeAuthority = true
			}
		}
	}
	if !ev.HasBlock {
		if v, ok := m["block"]; ok {
			if n, ok := toInt64(v); ok && n >= 0 {
				ev.Block = uint64(n)
				ev.HasBlock = true
			}
		}
	}
	if !ev.HasTimestamp {
		if v, ok := m["timestamp"]; ok {
			if n, ok := toInt64(v); ok {
				ev.TimestampMs = n
				ev.HasTimestamp = true
			}
		}
	}
	if v, ok := m["initialBuy"]; ok {
		if f, ok := toFloat64(v); ok {
			ev.InitialBuy = f
			ev.HasInitialBuy = true
			if !ev.HasTokenAmount {
				ev.TokenAmount = f
				ev.HasTokenAmount = true
			}
		}
	}
}

func parsePctString(s string) (pct float64, ok bool) {
	t := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "%"))
	if t == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(t, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func topString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return ""
	}
	return strings.TrimSpace(s)
}

func findFirstString(v interface{}, keys []string) string {
	switch t := v.(type) {
	case map[string]interface{}:
		for _, k := range keys {
			if val, ok := t[k]; ok {
				if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
					return s
				}
			}
		}
		for _, val := range t {
			if s := findFirstString(val, keys); s != "" {
				return s
			}
		}
	case []interface{}:
		for _, el := range t {
			if s := findFirstString(el, keys); s != "" {
				return s
			}
		}
	}
	return ""
}

func findFirstFloat(v interface{}, keys []string) (float64, bool) {
	switch t := v.(type) {
	case map[string]interface{}:
		for _, k := range keys {
			if val, ok := t[k]; ok {
				if f, ok := toFloat64(val); ok {
					return f, true
				}
			}
		}
		for _, val := range t {
			if f, has := findFirstFloat(val, keys); has {
				return f, true
			}
		}
	case []interface{}:
		for _, el := range t {
			if f, has := findFirstFloat(el, keys); has {
				return f, true
			}
		}
	}
	return 0, false
}

func findFirstInt64ish(v interface{}, keys []string) (int64, bool) {
	switch t := v.(type) {
	case map[string]interface{}:
		for _, k := range keys {
			if val, ok := t[k]; ok {
				if n, ok := toInt64(val); ok {
					return n, true
				}
			}
		}
		for _, val := range t {
			if n, has := findFirstInt64ish(val, keys); has {
				return n, true
			}
		}
	case []interface{}:
		for _, el := range t {
			if n, has := findFirstInt64ish(el, keys); has {
				return n, true
			}
		}
	}
	return 0, false
}

func toFloat64(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func toInt64(v interface{}) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int64:
		return x, true
	case float64:
		return int64(x), true
	case json.Number:
		i, err := x.Int64()
		return i, err == nil
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		return i, err == nil
	default:
		return 0, false
	}
}
