package store

import (
	"context"
	"testing"

	"rlangga/internal/redisx"
	"rlangga/internal/testutil"
)

func TestSaveTrade_LoadRecent_Dedupe(t *testing.T) {
	testutil.UseMiniredis(t)

	tr := Trade{
		Mint: "m1", BuySOL: 0.1, SellSOL: 0.11, PnLSOL: 0.01, Percent: 10, DurationSec: 5, TS: 100,
	}
	if err := SaveTrade(tr); err != nil {
		t.Fatal(err)
	}
	if err := SaveTrade(tr); err != nil {
		t.Fatal(err)
	}

	got, err := LoadRecent(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("dedupe expected 1 entry, got %d", len(got))
	}
	if got[0].Mint != "m1" || got[0].PnLSOL != 0.01 {
		t.Fatalf("%+v", got[0])
	}
}

func TestLoadRecent_SkipsBadJSON(t *testing.T) {
	testutil.UseMiniredis(t)
	_ = redisx.Client.LPush(context.Background(), keyTradesList, "{").Err()
	got, err := LoadRecent(5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d", len(got))
	}
}

func TestSaveTrade_NoRedis(t *testing.T) {
	prev := redisx.Client
	redisx.Client = nil
	t.Cleanup(func() { redisx.Client = prev })
	if err := SaveTrade(Trade{Mint: "x"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadRecent_ZeroN(t *testing.T) {
	testutil.UseMiniredis(t)
	got, err := LoadRecent(0)
	if err != nil || len(got) != 0 {
		t.Fatalf("err=%v len=%d", err, len(got))
	}
}

func TestLoadRecent_NoRedis(t *testing.T) {
	prev := redisx.Client
	redisx.Client = nil
	t.Cleanup(func() { redisx.Client = prev })
	if _, err := LoadRecent(3); err == nil {
		t.Fatal("expected error")
	}
}
