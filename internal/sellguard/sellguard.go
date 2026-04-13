package sellguard

import (
	"context"
	"time"

	"rlangga/internal/redisx"
)

const keyPrefix = "position:exiting:"

const sellExitTTL = 5 * time.Minute

// TryAcquireSellExit sets a short-lived Redis lock so monitor and recovery cannot sell the same mint concurrently.
// Returns false if another holder already has the key. No Redis → true (cannot coordinate).
func TryAcquireSellExit(mint string) bool {
	if redisx.Client == nil {
		return true
	}
	ctx := context.Background()
	ok, err := redisx.Client.SetNX(ctx, keyPrefix+mint, "1", sellExitTTL).Result()
	return err == nil && ok
}

// ReleaseSellExit removes the lock after a sell attempt completes.
func ReleaseSellExit(mint string) {
	if redisx.Client == nil {
		return
	}
	_ = redisx.Client.Del(context.Background(), keyPrefix+mint).Err()
}
