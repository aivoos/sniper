package lock

import (
	"context"
	"errors"
	"time"

	"rlangga/internal/config"
	"rlangga/internal/redisx"
)

const mintKeyPrefix = "mint:"

func mintLockTTL() time.Duration {
	if config.C == nil {
		return 5 * time.Minute
	}
	if config.C.LockTTLMin > 0 {
		return time.Duration(config.C.LockTTLMin) * time.Minute
	}
	return 12 * time.Minute
}

// LockMint acquires a distributed lock for one mint (TTL dari LOCK_TTL_MIN / default; hazards §5).
func LockMint(mint string) bool {
	if redisx.Client == nil {
		return false
	}
	ctx := context.Background()
	ok, err := redisx.Client.SetNX(ctx, mintKeyPrefix+mint, "1", mintLockTTL()).Result()
	return err == nil && ok
}

// RefreshMint extends the TTL of an existing lock (keep-alive while monitoring).
func RefreshMint(mint string) {
	if redisx.Client == nil {
		return
	}
	_ = redisx.Client.Expire(context.Background(), mintKeyPrefix+mint, mintLockTTL()).Err()
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
