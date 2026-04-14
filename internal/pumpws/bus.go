package pumpws

import "sync"

// Simple in-process pubsub for StreamEvent keyed by mint.
// Used by monitor to react to rug-related events (e.g. liquidity remove) in near real-time.

var (
	busMu sync.RWMutex
	bus   = map[string]map[chan StreamEvent]struct{}{}
)

// PublishStreamEvent broadcasts ev to all subscribers for ev.Mint.
// Non-blocking: slow subscribers may drop events.
func PublishStreamEvent(ev StreamEvent) {
	if ev.Mint == "" {
		return
	}
	// Hold RLock for the full fan-out: subs is a map that cancel() mutates; iterating
	// after Unlock races with delete/close (see monitor whale_exit_test with -race).
	busMu.RLock()
	defer busMu.RUnlock()
	subs := bus[ev.Mint]
	if len(subs) == 0 {
		return
	}
	for ch := range subs {
		select {
		case ch <- ev:
		default:
			// drop
		}
	}
}

// SubscribeMint subscribes to events for a specific mint. Caller must call cancel to unsubscribe.
func SubscribeMint(mint string) (ch <-chan StreamEvent, cancel func()) {
	if mint == "" {
		c := make(chan StreamEvent)
		close(c)
		return c, func() {}
	}
	c := make(chan StreamEvent, 16)
	busMu.Lock()
	m := bus[mint]
	if m == nil {
		m = map[chan StreamEvent]struct{}{}
		bus[mint] = m
	}
	m[c] = struct{}{}
	busMu.Unlock()

	return c, func() {
		busMu.Lock()
		if mm := bus[mint]; mm != nil {
			delete(mm, c)
			if len(mm) == 0 {
				delete(bus, mint)
			}
		}
		busMu.Unlock()
		close(c)
	}
}
