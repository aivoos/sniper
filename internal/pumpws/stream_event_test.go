package pumpws

import "testing"

func TestParseStreamEvent_TxAndSol(t *testing.T) {
	msg := []byte(`{"txType":"create","mint":"So11111111111111111111111111111111111111112","solAmount":1.5}`)
	ev, ok := ParseStreamEvent(msg)
	if !ok {
		t.Fatal("parse")
	}
	if ev.TxType != "create" {
		t.Fatalf("txType %q", ev.TxType)
	}
	if ev.Mint != "So11111111111111111111111111111111111111112" {
		t.Fatalf("mint %q", ev.Mint)
	}
	if !ev.HasSolAmount || ev.SolAmount != 1.5 {
		t.Fatalf("sol %+v", ev)
	}
}

func TestParseStreamEvent_Lamports(t *testing.T) {
	msg := []byte(`{"mint":"So11111111111111111111111111111111111111112","lamports":1500000000}`)
	ev, ok := ParseStreamEvent(msg)
	if !ok {
		t.Fatal("parse")
	}
	if !ev.HasSolAmount || ev.SolAmount != 1.5 {
		t.Fatalf("sol %+v", ev)
	}
}

func TestParseStreamEvent_Nested(t *testing.T) {
	msg := []byte(`{"data":{"type":"buy","mint":"So11111111111111111111111111111111111111112"}}`)
	ev, ok := ParseStreamEvent(msg)
	if !ok {
		t.Fatal("parse")
	}
	if ev.TxType != "buy" {
		t.Fatalf("txType %q", ev.TxType)
	}
}

func TestParseStreamEvent_InvalidJSON(t *testing.T) {
	_, ok := ParseStreamEvent([]byte(`not json`))
	if ok {
		t.Fatal("expected false")
	}
}

// Gaya PumpPortal: mint + signature tipis; txType hanya jika vendor mengirim key yang dikenali.
func TestParseStreamEvent_PumpPortalFlat_minimal(t *testing.T) {
	msg := []byte(`{"mint":"So11111111111111111111111111111111111111112","signature":"58zv6eEs2Y9ARPt9VSdpo6h3A4sg2ijgNftk8vXGvjoHQEiMqgoL6mNnWX9uZ26WS6mtzWuXduf8vuhUwUKJ73Wk"}`)
	ev, ok := ParseStreamEvent(msg)
	if !ok {
		t.Fatal("parse")
	}
	if ev.Mint != "So11111111111111111111111111111111111111112" {
		t.Fatalf("mint %q", ev.Mint)
	}
	if ev.Signature == "" {
		t.Fatal("signature")
	}
	if ev.TxType != "" {
		t.Fatalf("txType should be empty for minimal portal-shaped msg, got %q", ev.TxType)
	}
}

// Gaya PumpPortal: method + params nested (txType dari "type" di dalam params).
func TestParseStreamEvent_PumpPortal_methodParams(t *testing.T) {
	msg := []byte(`{"method":"tokenTrade","params":{"mint":"So11111111111111111111111111111111111111112","type":"buy","solAmount":1.2}}`)
	ev, ok := ParseStreamEvent(msg)
	if !ok {
		t.Fatal("parse")
	}
	if ev.TxType != "buy" {
		t.Fatalf("txType %q", ev.TxType)
	}
	if ev.Method != "tokentrade" {
		t.Fatalf("method %q", ev.Method)
	}
	if !ev.HasSolAmount || ev.SolAmount != 1.2 {
		t.Fatalf("sol %+v", ev)
	}
}

// Pool / marketCapSol di params (bukan root) — sama dipakai filter WSS untuk primer + fallback WSS.
func TestParseStreamEvent_NestedParams_poolMcap(t *testing.T) {
	msg := []byte(`{"method":"subscribeNewToken","params":{"mint":"So11111111111111111111111111111111111111112","txType":"create","pool":"pump","marketCapSol":42.5,"block":100}}`)
	ev, ok := ParseStreamEvent(msg)
	if !ok {
		t.Fatal("parse")
	}
	if ev.Pool != "pump" {
		t.Fatalf("pool %q", ev.Pool)
	}
	if !ev.HasMarketCapSOL || ev.MarketCapSOL != 42.5 {
		t.Fatalf("mcap %+v", ev)
	}
	if !ev.HasBlock || ev.Block != 100 {
		t.Fatalf("block %+v", ev)
	}
}

// TxType harus dari txType, bukan dari field "name" (nama token di event create PumpAPI).
func TestParseStreamEvent_TxTypeNotTokenName(t *testing.T) {
	msg := []byte(`{"name":"CHUD","symbol":"CHUD","txType":"create","mint":"So11111111111111111111111111111111111111112","solAmount":3}`)
	ev, ok := ParseStreamEvent(msg)
	if !ok {
		t.Fatal("parse")
	}
	if ev.TxType != "create" {
		t.Fatalf("txType %q want create (not token name)", ev.TxType)
	}
}

// Bentuk mirip stream PumpAPI: root + marketCapSol + initialBuy.
func TestParseStreamEvent_PumpAPIStyle(t *testing.T) {
	msg := []byte(`{"txType":"add","mint":"AvxohnS3SSJRfw4h9u2am5DTRrNv9HY5je7EdqpVSA2i","solAmount":3,"marketCapSol":33.83,"pool":"pump-amm","tokensInPool":561786808.85806,"solInPool":42.052216573,"poolFeeRate":0.012,"poolCreatedBy":"pump","burnedLiquidity":"100%","mintAuthority":null,"freezeAuthority":null,"tokenProgram":"spl-token-2022","tokenExtensions":{"metadataPointer":{},"tokenMetadata":{}},"initialBuy":97545454.545454,"block":401114006,"timestamp":1771427621579}`)
	ev, ok := ParseStreamEvent(msg)
	if !ok {
		t.Fatal("parse")
	}
	if ev.TxType != "add" || ev.Pool != "pump-amm" {
		t.Fatalf("tx=%q pool=%q", ev.TxType, ev.Pool)
	}
	if !ev.HasMarketCapSOL || ev.MarketCapSOL < 33 || ev.MarketCapSOL > 34 {
		t.Fatalf("mcap %v", ev.MarketCapSOL)
	}
	if !ev.HasInitialBuy || ev.InitialBuy < 9e7 {
		t.Fatalf("initialBuy %+v", ev)
	}
	if !ev.HasTokenAmount || ev.TokenAmount < 9e7 {
		t.Fatalf("initialBuy mirrored to token %+v", ev)
	}
	if !ev.HasBlock || ev.Block != 401114006 {
		t.Fatalf("block %v", ev.Block)
	}
	if !ev.HasTimestamp || ev.TimestampMs != 1771427621579 {
		t.Fatalf("ts %v", ev.TimestampMs)
	}
	if !ev.HasTokensInPool || ev.TokensInPool < 5e8 {
		t.Fatalf("tokensInPool %+v", ev)
	}
	if !ev.HasSolInPool || ev.SolInPool < 40 {
		t.Fatalf("solInPool %+v", ev)
	}
	if ev.PoolCreatedBy != "pump" {
		t.Fatalf("poolCreatedBy %q", ev.PoolCreatedBy)
	}
	if !ev.HasBurnedLiquidity || ev.BurnedLiquidityPct != 100 {
		t.Fatalf("burned %+v", ev)
	}
	if !ev.HasPoolFeeRate || ev.PoolFeeRate < 0.011 || ev.PoolFeeRate > 0.013 {
		t.Fatalf("poolFeeRate %+v", ev)
	}
	if ev.TokenProgram != "spl-token-2022" {
		t.Fatalf("tokenProgram %q", ev.TokenProgram)
	}
	if ev.HasMintAuthority || ev.HasFreezeAuthority {
		t.Fatalf("authorities %+v", ev)
	}
	if len(ev.Extensions) == 0 {
		t.Fatalf("extensions %+v", ev)
	}
}
