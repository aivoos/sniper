package pumpws

import (
	"testing"
	"time"
)

func TestTrackEvent_BuySellRatio(t *testing.T) {
	ResetTracker()
	mint := "testMint1"
	for i := 0; i < 5; i++ {
		TrackEvent(StreamEvent{Mint: mint, TxType: "buy", SolAmount: 1.0, HasSolAmount: true})
	}
	for i := 0; i < 2; i++ {
		TrackEvent(StreamEvent{Mint: mint, TxType: "sell", SolAmount: 0.5, HasSolAmount: true})
	}
	act := GetMintActivity(mint, 30*time.Second)
	if act == nil {
		t.Fatal("expected activity")
	}
	if act.Buys != 5 {
		t.Fatalf("buys=%d want 5", act.Buys)
	}
	if act.Sells != 2 {
		t.Fatalf("sells=%d want 2", act.Sells)
	}
	ratio := act.BuySellRatio()
	if ratio != 2.5 {
		t.Fatalf("ratio=%.2f want 2.5", ratio)
	}
}

func TestTrackEvent_AgeSec(t *testing.T) {
	ResetTracker()
	mint := "testMint2"
	TrackEvent(StreamEvent{Mint: mint, TxType: "buy", SolAmount: 1})
	act := GetMintActivity(mint, 30*time.Second)
	if act == nil {
		t.Fatal("expected activity")
	}
	if act.AgeSec() > 1 {
		t.Fatalf("age=%.2f, expected <1s", act.AgeSec())
	}
}

func TestTrackEvent_McapRising(t *testing.T) {
	ResetTracker()
	mint := "testMint3"
	TrackEvent(StreamEvent{Mint: mint, TxType: "buy", MarketCapSOL: 100, HasMarketCapSOL: true})
	TrackEvent(StreamEvent{Mint: mint, TxType: "buy", MarketCapSOL: 120, HasMarketCapSOL: true})
	act := GetMintActivity(mint, 30*time.Second)
	if act == nil {
		t.Fatal("expected activity")
	}
	if !act.McapRising() {
		t.Fatal("expected mcap rising")
	}
}

func TestTrackEvent_McapFalling(t *testing.T) {
	ResetTracker()
	mint := "testMint4"
	TrackEvent(StreamEvent{Mint: mint, TxType: "buy", MarketCapSOL: 100, HasMarketCapSOL: true})
	TrackEvent(StreamEvent{Mint: mint, TxType: "sell", MarketCapSOL: 80, HasMarketCapSOL: true})
	act := GetMintActivity(mint, 30*time.Second)
	if act == nil {
		t.Fatal("expected activity")
	}
	if act.McapRising() {
		t.Fatal("expected mcap falling")
	}
}

func TestTrackEvent_NoActivity(t *testing.T) {
	ResetTracker()
	act := GetMintActivity("nonexistent", 30*time.Second)
	if act != nil {
		t.Fatal("expected nil")
	}
}

func TestTrackEvent_IgnoresNonBuySell(t *testing.T) {
	ResetTracker()
	mint := "testMint5"
	TrackEvent(StreamEvent{Mint: mint, TxType: "create"})
	TrackEvent(StreamEvent{Mint: mint, TxType: "remove"})
	act := GetMintActivity(mint, 30*time.Second)
	if act != nil {
		t.Fatal("expected nil for non buy/sell events")
	}
}

func TestBuySellRatio_ZeroSells(t *testing.T) {
	ResetTracker()
	mint := "testMint6"
	TrackEvent(StreamEvent{Mint: mint, TxType: "buy", SolAmount: 1})
	TrackEvent(StreamEvent{Mint: mint, TxType: "buy", SolAmount: 2})
	act := GetMintActivity(mint, 30*time.Second)
	if act.BuySellRatio() != 2.0 {
		t.Fatalf("ratio=%.2f want 2.0 (buys as float)", act.BuySellRatio())
	}
}

func TestPruneTracker(t *testing.T) {
	ResetTracker()
	TrackEvent(StreamEvent{Mint: "old", TxType: "buy"})
	PruneTracker(0)
	act := GetMintActivity("old", 30*time.Second)
	if act != nil {
		t.Fatal("expected pruned")
	}
}
