package idempotency

import (
	"context"
	"time"

	"rlangga/internal/redisx"
)

const eventKeyPrefix = "event:"

// IsDuplicate returns true if this mint was seen recently (dedupe window 10s).
func IsDuplicate(mint string) bool {
	if redisx.Client == nil {
		return false
	}
	ctx := context.Background()
	ok, err := redisx.Client.SetNX(ctx, eventKeyPrefix+mint, "1", 10*time.Second).Result()
	if err != nil {
		return false
	}
	return !ok
}
