package lock

import (
	"context"
	"testing"
	"time"

	"rlangga/internal/config"
	"rlangga/internal/redisx"
	"rlangga/internal/testutil"
)

func TestLockMint_AcquireRelease(t *testing.T) {
	testutil.UseMiniredis(t)
	if !LockMint("mintA") {
		t.Fatal("first lock should succeed")
	}
	if LockMint("mintA") {
		t.Fatal("second lock same mint should fail")
	}
	UnlockMint("mintA")
	if !LockMint("mintA") {
		t.Fatal("lock after unlock should succeed")
	}
}

func TestLockMint_NoRedis(t *testing.T) {
	if LockMint("x") {
		t.Fatal("without redis should not acquire")
	}
}

func TestPing_NoRedis(t *testing.T) {
	if err := Ping(); err == nil {
		t.Fatal("expected error without redis")
	}
}

func TestPing_OK(t *testing.T) {
	testutil.UseMiniredis(t)
	if err := Ping(); err != nil {
		t.Fatal(err)
	}
}

func TestUnlockMint_NoRedis(t *testing.T) {
	UnlockMint("x")
}

func TestLockMint_UsesConfigTTLMinutes(t *testing.T) {
	testutil.UseMiniredis(t)
	t.Setenv("RPC_STUB", "1")
	t.Setenv("LOCK_TTL_MIN", "2")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if !LockMint("ttlMint") {
		t.Fatal("expected lock")
	}
	ctx := context.Background()
	pt, err := redisx.Client.PTTL(ctx, "mint:ttlMint").Result()
	if err != nil {
		t.Fatal(err)
	}
	if pt < 60*time.Second || pt > 150*time.Second {
		t.Fatalf("PTTL=%v, want ~2 minutes", pt)
	}
}

func TestLockMint_LockTTLMinZeroUsesTwelveMinutes(t *testing.T) {
	testutil.UseMiniredis(t)
	t.Setenv("RPC_STUB", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	config.C.LockTTLMin = 0
	if !LockMint("zeroTTL") {
		t.Fatal("expected lock")
	}
	pt, err := redisx.Client.PTTL(context.Background(), "mint:zeroTTL").Result()
	if err != nil {
		t.Fatal(err)
	}
	if pt < 11*time.Minute || pt > 13*time.Minute {
		t.Fatalf("PTTL=%v, want ~12 minutes when LockTTLMin=0", pt)
	}
}
