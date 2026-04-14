package positionlimit

import (
	"context"
	"testing"

	"rlangga/internal/redisx"
	"rlangga/internal/testutil"
)

func clearLocal(t *testing.T) {
	t.Helper()
	for Open() > 0 {
		Release("cleanup")
	}
}

func TestTryReserve_Unlimited(t *testing.T) {
	clearLocal(t)
	if !TryReserve(0, "m1") {
		t.Fatal("max<=0 means unlimited")
	}
}

func TestTryReserve_EmptyMint(t *testing.T) {
	if TryReserve(1, "") {
		t.Fatal("empty mint rejected")
	}
}

func TestTryReserve_Local(t *testing.T) {
	clearLocal(t)
	if !TryReserve(2, "a") {
		t.Fatal("first slot")
	}
	if !TryReserve(2, "b") {
		t.Fatal("second slot")
	}
	if TryReserve(2, "c") {
		t.Fatal("third should fail")
	}
	Release("a")
	if !TryReserve(2, "c") {
		t.Fatal("after release")
	}
	if Open() != 2 {
		t.Fatalf("open: %d", Open())
	}
	Release("b")
	Release("c")
	if Open() != 0 {
		t.Fatalf("open after release: %d", Open())
	}
}

func TestTryReserve_Redis(t *testing.T) {
	testutil.UseMiniredis(t)
	if err := ResetRedisState(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !TryReserve(2, "x") || !TryReserve(2, "y") {
		t.Fatal("reserve two")
	}
	if TryReserve(2, "z") {
		t.Fatal("third fails")
	}
	ms, err := ActiveMints()
	if err != nil || len(ms) != 2 {
		t.Fatalf("mints: %v err=%v", ms, err)
	}
	Release("x")
	if !TryReserve(2, "z") {
		t.Fatal("after redis release")
	}
	_ = ResetRedisState(context.Background())
}

func TestOpen_RedisInvalidCount(t *testing.T) {
	testutil.UseMiniredis(t)
	_ = ResetRedisState(context.Background())
	if err := redisx.Client.Set(context.Background(), "rlangga:pos:open_count", "nope", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if Open() != -1 {
		t.Fatalf("expected -1, got %d", Open())
	}
	_ = ResetRedisState(context.Background())
}
