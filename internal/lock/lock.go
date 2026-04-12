package lock

import (
	"context"
	"errors"
	"time"

	"rlangga/internal/redisx"
)

const mintKeyPrefix = "mint:"

// LockMint acquires a distributed lock for one mint (default TTL 5 minutes).
func LockMint(mint string) bool {
	if redisx.Client == nil {
		return false
	}
	ctx := context.Background()
	ok, err := redisx.Client.SetNX(ctx, mintKeyPrefix+mint, "1", 5*time.Minute).Result()
	return err == nil && ok
}

// UnlockMint releases the lock for mint.
func UnlockMint(mint string) {
	if redisx.Client == nil {
		return
	}
	_ = redisx.Client.Del(context.Background(), mintKeyPrefix+mint).Err()
}

// Ping verifies Redis (optional health check).
func Ping() error {
	if redisx.Client == nil {
		return errors.New("lock: redis not initialized")
	}
	return redisx.Client.Ping(context.Background()).Err()
}
