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

func TestIsDuplicate_NoRedis(t *testing.T) {
	if IsDuplicate("m") {
		t.Fatal("without redis should not treat as duplicate")
	}
}

func TestIsDuplicate_RedisError(t *testing.T) {
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
	if IsDuplicate("m") {
		t.Fatal("redis error should not count as duplicate")
	}
}
