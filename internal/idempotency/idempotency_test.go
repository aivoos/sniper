package idempotency

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"rlangga/internal/redisx"
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

func TestIsDuplicate_NoRedis_FailClosed(t *testing.T) {
	// Fail-closed: no Redis means treat as duplicate (block trade)
	if !IsDuplicate("m") {
		t.Fatal("without redis should treat as duplicate (fail-closed)")
	}
}

func TestIsDuplicate_RedisError_FailClosed(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	redisx.Client = redis.NewClient(&redis.Options{Addr: s.Addr()})
	s.Close()
	t.Cleanup(func() {
		if redisx.Client != nil {
			_ = redisx.Client.Close()
		}
		redisx.Client = nil
	})
	// Fail-closed: Redis error means treat as duplicate (block trade)
	if !IsDuplicate("m") {
		t.Fatal("redis error should treat as duplicate (fail-closed)")
	}
}
