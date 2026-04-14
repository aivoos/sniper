// Package positionlimit membatasi jumlah posisi terbuka bersamaan.
// Jika Redis tersedia, counter + SET mint disimpan di Redis (rlangga:pos:*)
// agar beberapa proses worker atau inspeksi manual konsisten.
package positionlimit

import (
	"context"
	"strconv"
	"sync/atomic"

	"github.com/redis/go-redis/v9"

	"rlangga/internal/log"
	"rlangga/internal/redisx"
)

const (
	redisKeyCount = "rlangga:pos:open_count"
	redisKeySet   = "rlangga:pos:mints"
)

var localOpen int64

var reserveLua = redis.NewScript(`
local max = tonumber(ARGV[1])
local mint = ARGV[2]
local c = tonumber(redis.call('GET', KEYS[1]) or '0')
if c >= max then return 0 end
redis.call('INCR', KEYS[1])
redis.call('SADD', KEYS[2], mint)
return 1
`)

var releaseLua = redis.NewScript(`
local mint = ARGV[1]
local n = tonumber(redis.call('GET', KEYS[1]) or '0')
if n > 0 then
  redis.call('DECR', KEYS[1])
end
redis.call('SREM', KEYS[2], mint)
return 1
`)

// TryReserve mengambil satu slot jika jumlah saat ini < max. max <= 0 berarti tak terbatas.
// mint wajib unik per posisi (sudah dijamin lock mint).
func TryReserve(max int, mint string) bool {
	if max <= 0 {
		return true
	}
	if mint == "" {
		return false
	}
	if redisx.Client != nil {
		n, err := reserveLua.Run(context.Background(), redisx.Client, []string{redisKeyCount, redisKeySet}, max, mint).Int()
		if err != nil {
			log.Error("positionlimit: TryReserve redis: " + err.Error())
			return false
		}
		return n == 1
	}
	return tryReserveLocal(max)
}

func tryReserveLocal(max int) bool {
	for {
		n := atomic.LoadInt64(&localOpen)
		if int(n) >= max {
			return false
		}
		if atomic.CompareAndSwapInt64(&localOpen, n, n+1) {
			return true
		}
	}
}

// Release mengembalikan satu slot untuk mint tersebut.
func Release(mint string) {
	if mint == "" {
		return
	}
	if redisx.Client != nil {
		_, err := releaseLua.Run(context.Background(), redisx.Client, []string{redisKeyCount, redisKeySet}, mint).Result()
		if err != nil {
			log.Error("positionlimit: Release redis: " + err.Error())
		}
		return
	}
	atomic.AddInt64(&localOpen, -1)
}

// Open mengembalikan jumlah slot terpakai (Redis GET atau counter lokal).
func Open() int64 {
	if redisx.Client != nil {
		s, err := redisx.Client.Get(context.Background(), redisKeyCount).Result()
		if err == redis.Nil || s == "" {
			return 0
		}
		if err != nil {
			return -1
		}
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return -1
		}
		return v
	}
	return atomic.LoadInt64(&localOpen)
}

// ResetRedisState menghapus kunci counter + SET mint di Redis (setelah crash atau drift).
func ResetRedisState(ctx context.Context) error {
	if redisx.Client == nil {
		return nil
	}
	pipe := redisx.Client.Pipeline()
	pipe.Del(ctx, redisKeyCount)
	pipe.Del(ctx, redisKeySet)
	_, err := pipe.Exec(ctx)
	return err
}

// ActiveMints mengembalikan mint yang tercatat di SET Redis (kosong jika tanpa Redis).
func ActiveMints() ([]string, error) {
	if redisx.Client == nil {
		return nil, nil
	}
	return redisx.Client.SMembers(context.Background(), redisKeySet).Result()
}
