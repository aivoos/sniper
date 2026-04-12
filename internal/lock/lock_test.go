package lock

import (
	"testing"

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
