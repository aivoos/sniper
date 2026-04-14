package store

import (
	"context"
	"path/filepath"
	"testing"

	"rlangga/internal/pumpws"
	"rlangga/internal/redisx"
	"rlangga/internal/testutil"
)

func TestSaveTrade_LoadRecent_Dedupe(t *testing.T) {
	testutil.UseMiniredis(t)

	tr := Trade{
		Mint: "m1", BuySOL: 0.1, SellSOL: 0.11, PnLSOL: 0.01, Percent: 10, DurationSec: 5, TS: 100,
	}
	saved, err := SaveTrade(tr)
	if err != nil || !saved {
		t.Fatalf("first save: err=%v saved=%v", err, saved)
	}
	saved2, err := SaveTrade(tr)
	if err != nil || saved2 {
		t.Fatalf("dedupe expected saved=false, got %v err=%v", saved2, err)
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
	if _, err := SaveTrade(Trade{Mint: "x"}); err == nil {
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

func TestSaveTrade_SQLiteMirror(t *testing.T) {
	testutil.UseMiniredis(t)
	dbPath := filepath.Join(t.TempDir(), "mirror.db")
	t.Cleanup(func() {
		if sqliteDB != nil {
			_ = sqliteDB.Close()
			sqliteDB = nil
		}
	})
	if err := InitTradeSQLite(dbPath); err != nil {
		t.Fatal(err)
	}
	ev := &pumpws.StreamEvent{InitialBuy: 1e8, HasInitialBuy: true, MarketCapSOL: 42, HasMarketCapSOL: true, Pool: "pump", PoolID: "pid1"}
	tr := Trade{
		Mint: "mintSql", BuySOL: 0.1, SellSOL: 0.11, PnLSOL: 0.01, Percent: 10, DurationSec: 4, TS: 300, ExitReason: "take_profit",
	}
	ApplyStreamEntryToTrade(&tr, ev)
	saved, err := SaveTrade(tr)
	if err != nil || !saved {
		t.Fatalf("save: err=%v saved=%v", err, saved)
	}
	var n int
	if err := sqliteDB.QueryRow(`SELECT COUNT(*) FROM trades WHERE mint = ? AND entry_pool = ?`, "mintSql", "pump").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want 1 row in sqlite, got %d", n)
	}
}
