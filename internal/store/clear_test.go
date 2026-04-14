package store

import (
	"context"
	"testing"

	"rlangga/internal/redisx"
	"rlangga/internal/testutil"
)

func TestClearTradesAndDedupe(t *testing.T) {
	testutil.UseMiniredis(t)
	ctx := context.Background()
	_, err := SaveTrade(Trade{Mint: "m", BuySOL: 0.1, SellSOL: 0.11, PnLSOL: 0.01, Percent: 10, TS: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := ClearTradesAndDedupe(ctx); err != nil {
		t.Fatal(err)
	}
	n, err := redisx.Client.LLen(ctx, keyTradesList).Result()
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("trades:list len: %d", n)
	}
}
