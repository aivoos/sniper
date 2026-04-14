package idempotency

import (
	"context"
	"time"

	"rlangga/internal/log"
	"rlangga/internal/redisx"
)

const (
	eventKeyPrefix = "event:"
	cooldownPrefix = "cooldown:"
)

// IsDuplicate returns true if this mint was seen recently (dedupe window 10s)
// OR if the mint is in cooldown after a previous trade.
// Fail-closed: returns true (blocks trade) on Redis errors.
func IsDuplicate(mint string) bool {
	if redisx.Client == nil {
		log.Error("idempotency: redis nil, fail-closed (blocking mint=" + mint + ")")
		return true
	}
	ctx := context.Background()

	if exists, err := redisx.Client.Exists(ctx, cooldownPrefix+mint).Result(); err == nil && exists > 0 {
		return true
	}

	ok, err := redisx.Client.SetNX(ctx, eventKeyPrefix+mint, "1", 10*time.Second).Result()
	if err != nil {
		log.Error("idempotency: redis error (fail-closed): " + err.Error() + " mint=" + mint)
		return true
	}
	return !ok
}

// SetCooldown permanently blacklists a mint that lost money — never trade it again.
func SetCooldown(mint string) {
	if redisx.Client == nil {
		return
	}
	ctx := context.Background()
	_ = redisx.Client.Set(ctx, cooldownPrefix+mint, "1", 0).Err()
}
