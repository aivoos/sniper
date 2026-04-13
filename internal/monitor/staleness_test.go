package monitor

import (
	"testing"
	"time"
)

func TestQuoteStale(t *testing.T) {
	if quoteStale(time.Time{}, 1000) {
		t.Fatal("zero time never stale")
	}
	if quoteStale(time.Now(), 0) {
		t.Fatal("maxAge 0 disables check")
	}
	past := time.Now().Add(-2 * time.Second)
	if !quoteStale(past, 1000) {
		t.Fatal("old sample should be stale")
	}
	if quoteStale(time.Now(), 10_000) {
		t.Fatal("fresh sample not stale")
	}
}
