package testutil

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"rlangga/internal/redisx"
)

// UseMiniredis replaces redisx.Client with an in-memory server for tests.
func UseMiniredis(t *testing.T) {
	t.Helper()
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	redisx.Client = redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() {
		if redisx.Client != nil {
			_ = redisx.Client.Close()
		}
		redisx.Client = nil
		s.Close()
	})
}
