package store

import (
	"context"
	"fmt"

	"rlangga/internal/redisx"
)

// ClearTradesAndDedupe menghapus daftar trade dan kunci dedupe SaveTrade (Redis).
// Dipakai untuk mengosongkan riwayat PnL agregat tanpa FLUSHDB seluruh instance.
func ClearTradesAndDedupe(ctx context.Context) error {
	if redisx.Client == nil {
		return fmt.Errorf("store: redis not initialized")
	}
	if err := redisx.Client.Del(ctx, keyTradesList).Err(); err != nil {
		return err
	}
	var cursor uint64
	pattern := prefixDedupe + "*"
	for {
		keys, next, err := redisx.Client.Scan(ctx, cursor, pattern, 500).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := redisx.Client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return nil
}
