package sellguard

import (
	"context"
	"testing"

	"rlangga/internal/redisx"
	"rlangga/internal/testutil"
)

func TestTryAcquireSellExit_SecondFails(t *testing.T) {
	testutil.UseMiniredis(t)
	mint := "So11111111111111111111111111111111111111112"
	if !TryAcquireSellExit(mint) {
		t.Fatal("first acquire")
	}
	if TryAcquireSellExit(mint) {
		t.Fatal("second acquire should fail")
	}
	ReleaseSellExit(mint)
	if !TryAcquireSellExit(mint) {
		t.Fatal("after release")
	}
}

func TestTryAcquireSellExit_NoRedis(t *testing.T) {
	prev := redisx.Client
	redisx.Client = nil
	t.Cleanup(func() { redisx.Client = prev })
	if !TryAcquireSellExit("x") {
		t.Fatal("no redis allows")
	}
}

func TestReleaseSellExit_NoRedis(t *testing.T) {
	prev := redisx.Client
	redisx.Client = nil
	t.Cleanup(func() { redisx.Client = prev })
	ReleaseSellExit("x") // no panic
}

func TestKeyPrefix(t *testing.T) {
	testutil.UseMiniredis(t)
	mint := "mintA"
	_ = TryAcquireSellExit(mint)
	ctx := context.Background()
	n, err := redisx.Client.Exists(ctx, keyPrefix+mint).Result()
	if err != nil || n != 1 {
		t.Fatalf("exists=%d err=%v", n, err)
	}
}
