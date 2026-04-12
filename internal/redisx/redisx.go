package redisx

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Client is the shared Redis handle (initialized in app.Init).
var Client *redis.Client

// Init parses addr like "host:6379" and connects.
func Init(addr string) error {
	if addr == "" {
		return fmt.Errorf("redisx: REDIS_URL is required")
	}
	Client = redis.NewClient(&redis.Options{Addr: addr})
	return Client.Ping(context.Background()).Err()
}
