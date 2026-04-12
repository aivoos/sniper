package idempotency

import (
	"testing"

	"rlangga/internal/testutil"
)

func TestIsDuplicate_WithinWindow(t *testing.T) {
	testutil.UseMiniredis(t)
	if IsDuplicate("m1") {
		t.Fatal("first event should not be duplicate")
	}
	if !IsDuplicate("m1") {
		t.Fatal("second event same mint should be duplicate")
	}
}

func TestIsDuplicate_NoRedis(t *testing.T) {
	if IsDuplicate("m") {
		t.Fatal("without redis should not treat as duplicate")
	}
}
