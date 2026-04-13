package filter

import (
	"testing"

	"rlangga/internal/config"
	"rlangga/internal/pumpws"
)

func TestAllowStreamEvent_NoGate(t *testing.T) {
	old := config.C
	t.Cleanup(func() { config.C = old })
	config.C = &config.Config{}
	ev := pumpws.StreamEvent{Mint: "So11111111111111111111111111111111111111112", TxType: "anything"}
	ok, reason := AllowStreamEvent(&ev)
	if !ok || reason != "" {
		t.Fatalf("got %v %q", ok, reason)
	}
}

func TestAllowStreamEvent_AllowTx(t *testing.T) {
	old := config.C
	t.Cleanup(func() { config.C = old })
	config.C = &config.Config{FilterWSSAllowTxTypes: []string{"create", "migrate"}}
	ev := pumpws.StreamEvent{Mint: "So11111111111111111111111111111111111111112", TxType: "create"}
	ok, reason := AllowStreamEvent(&ev)
	if !ok {
		t.Fatal(reason)
	}
	ev.TxType = "sell"
	ok, reason = AllowStreamEvent(&ev)
	if ok {
		t.Fatal("expected block")
	}
	if reason == "" {
		t.Fatal("expected reason")
	}
}

func TestAllowStreamEvent_DenyTx(t *testing.T) {
	old := config.C
	t.Cleanup(func() { config.C = old })
	config.C = &config.Config{FilterWSSDenyTxTypes: []string{"sell"}}
	ev := pumpws.StreamEvent{Mint: "So11111111111111111111111111111111111111112", TxType: "buy"}
	if ok, _ := AllowStreamEvent(&ev); !ok {
		t.Fatal("expected allow")
	}
	ev.TxType = "sell"
	if ok, _ := AllowStreamEvent(&ev); ok {
		t.Fatal("expected deny")
	}
}

func TestAllowStreamEvent_MinSOL(t *testing.T) {
	old := config.C
	t.Cleanup(func() { config.C = old })
	config.C = &config.Config{FilterWSSMinSOL: 2}
	ev := pumpws.StreamEvent{Mint: "So11111111111111111111111111111111111111112", SolAmount: 1, HasSolAmount: true}
	if ok, _ := AllowStreamEvent(&ev); ok {
		t.Fatal("expected block below min")
	}
	ev.SolAmount = 3
	if ok, _ := AllowStreamEvent(&ev); !ok {
		t.Fatal("expected allow")
	}
}

func TestAllowStreamEvent_PoolAllow(t *testing.T) {
	old := config.C
	t.Cleanup(func() { config.C = old })
	config.C = &config.Config{FilterWSSPoolAllow: []string{"pump", "raydium"}}
	ev := pumpws.StreamEvent{Mint: "So11111111111111111111111111111111111111112", Pool: "pump"}
	if ok, _ := AllowStreamEvent(&ev); !ok {
		t.Fatal("expected allow")
	}
	ev.Pool = "other"
	if ok, reason := AllowStreamEvent(&ev); ok || reason == "" {
		t.Fatalf("expected block: ok=%v reason=%q", ok, reason)
	}
}

func TestAllowStreamEvent_MarketCapRange(t *testing.T) {
	old := config.C
	t.Cleanup(func() { config.C = old })
	config.C = &config.Config{
		FilterWSSMinMarketCapSOL: 10,
		FilterWSSMaxMarketCapSOL: 100,
	}
	ev := pumpws.StreamEvent{Mint: "So11111111111111111111111111111111111111112", MarketCapSOL: 5, HasMarketCapSOL: true}
	if ok, _ := AllowStreamEvent(&ev); ok {
		t.Fatal("expected below min")
	}
	ev.MarketCapSOL = 50
	if ok, _ := AllowStreamEvent(&ev); !ok {
		t.Fatal("expected allow in range")
	}
	ev.MarketCapSOL = 200
	if ok, _ := AllowStreamEvent(&ev); ok {
		t.Fatal("expected above max")
	}
}

func TestAllowStreamEvent_PoolCreatedByAllow(t *testing.T) {
	old := config.C
	t.Cleanup(func() { config.C = old })
	config.C = &config.Config{FilterWSSRequirePoolCreatedBy: []string{"pump", "raydium-launchpad"}}

	ev := pumpws.StreamEvent{Mint: "m", PoolCreatedBy: "pump"}
	if ok, _ := AllowStreamEvent(&ev); !ok {
		t.Fatal("expected allow")
	}
	ev.PoolCreatedBy = "custom"
	if ok, reason := AllowStreamEvent(&ev); ok || reason == "" {
		t.Fatalf("expected block: ok=%v reason=%q", ok, reason)
	}
}

func TestAllowStreamEvent_MinBurnedLiquidity(t *testing.T) {
	old := config.C
	t.Cleanup(func() { config.C = old })
	config.C = &config.Config{FilterWSSMinBurnedLiquidityPct: 100}

	ev := pumpws.StreamEvent{Mint: "m", HasBurnedLiquidity: true, BurnedLiquidityPct: 50}
	if ok, _ := AllowStreamEvent(&ev); ok {
		t.Fatal("expected block below min")
	}
	ev.BurnedLiquidityPct = 100
	if ok, _ := AllowStreamEvent(&ev); !ok {
		t.Fatal("expected allow")
	}
}

func TestAllowStreamEvent_MaxPoolFeeRate(t *testing.T) {
	old := config.C
	t.Cleanup(func() { config.C = old })
	config.C = &config.Config{FilterWSSMaxPoolFeeRate: 0.001}

	ev := pumpws.StreamEvent{Mint: "m", HasPoolFeeRate: true, PoolFeeRate: 0.01}
	if ok, _ := AllowStreamEvent(&ev); ok {
		t.Fatal("expected block fee above max")
	}
	ev.PoolFeeRate = 0.0005
	if ok, _ := AllowStreamEvent(&ev); !ok {
		t.Fatal("expected allow")
	}
}

func TestAllowStreamEvent_TokenAuthorityAndExtensions(t *testing.T) {
	old := config.C
	t.Cleanup(func() { config.C = old })
	config.C = &config.Config{
		FilterWSSRejectMintAuthority:   true,
		FilterWSSRejectFreezeAuthority: true,
		FilterWSSDenyTokenExtensions:   []string{"transferfeeconfig", "transferhook"},
	}
	ev := pumpws.StreamEvent{Mint: "m", HasMintAuthority: true}
	if ok, reason := AllowStreamEvent(&ev); ok || reason == "" {
		t.Fatalf("expected mintAuthority block: ok=%v reason=%q", ok, reason)
	}
	ev = pumpws.StreamEvent{Mint: "m", HasFreezeAuthority: true}
	if ok, reason := AllowStreamEvent(&ev); ok || reason == "" {
		t.Fatalf("expected freezeAuthority block: ok=%v reason=%q", ok, reason)
	}
	ev = pumpws.StreamEvent{Mint: "m", Extensions: []string{"metadataPointer", "transferFeeConfig"}}
	if ok, reason := AllowStreamEvent(&ev); ok || reason == "" {
		t.Fatalf("expected extension deny: ok=%v reason=%q", ok, reason)
	}
	ev = pumpws.StreamEvent{Mint: "m", Extensions: []string{"metadataPointer"}}
	if ok, _ := AllowStreamEvent(&ev); !ok {
		t.Fatal("expected allow safe extension")
	}
}
