package pumpws

import (
	"sync"
	"time"
)

// MintActivity stores aggregated buy/sell activity for a single mint over a sliding window.
type MintActivity struct {
	Buys       int
	Sells      int
	BuySOL     float64
	SellSOL    float64
	FirstSeen  time.Time
	LastMcap   float64
	PrevMcap   float64
	HasMcap    bool
}

// BuySellRatio returns buys/sells. If sells==0, returns buys as float (max pressure).
func (a *MintActivity) BuySellRatio() float64 {
	if a.Sells == 0 {
		return float64(a.Buys)
	}
	return float64(a.Buys) / float64(a.Sells)
}

// AgeSec returns seconds since first seen.
func (a *MintActivity) AgeSec() float64 {
	if a.FirstSeen.IsZero() {
		return 0
	}
	return time.Since(a.FirstSeen).Seconds()
}

// McapRising returns true if last mcap > previous mcap (price going up).
func (a *MintActivity) McapRising() bool {
	return a.HasMcap && a.LastMcap > a.PrevMcap && a.PrevMcap > 0
}

type trackedEvent struct {
	ts     time.Time
	isBuy  bool
	sol    float64
	mcap   float64
	hasMC  bool
}

var (
	trackerMu sync.RWMutex
	tracker   = map[string]*mintTracker{}
)

type mintTracker struct {
	firstSeen time.Time
	events    []trackedEvent
}

// TrackEvent records a stream event for activity analysis.
// Call this for every parsed StreamEvent (before filtering).
func TrackEvent(ev StreamEvent) {
	if ev.Mint == "" {
		return
	}
	now := time.Now()
	isBuy := ev.TxType == "buy"
	isSell := ev.TxType == "sell"
	if !isBuy && !isSell {
		return
	}

	te := trackedEvent{
		ts:    now,
		isBuy: isBuy,
		sol:   ev.SolAmount,
		mcap:  ev.MarketCapSOL,
		hasMC: ev.HasMarketCapSOL,
	}

	trackerMu.Lock()
	mt := tracker[ev.Mint]
	if mt == nil {
		mt = &mintTracker{firstSeen: now}
		tracker[ev.Mint] = mt
	}
	mt.events = append(mt.events, te)
	trackerMu.Unlock()
}

// GetMintActivity returns aggregated activity for a mint within the given window.
func GetMintActivity(mint string, window time.Duration) *MintActivity {
	trackerMu.RLock()
	mt := tracker[mint]
	trackerMu.RUnlock()
	if mt == nil {
		return nil
	}

	cutoff := time.Now().Add(-window)
	a := &MintActivity{FirstSeen: mt.firstSeen}

	trackerMu.RLock()
	for _, e := range mt.events {
		if e.ts.Before(cutoff) {
			continue
		}
		if e.isBuy {
			a.Buys++
			a.BuySOL += e.sol
		} else {
			a.Sells++
			a.SellSOL += e.sol
		}
		if e.hasMC {
			a.PrevMcap = a.LastMcap
			a.LastMcap = e.mcap
			a.HasMcap = true
		}
	}
	trackerMu.RUnlock()
	return a
}

// PruneTracker removes stale mints not seen for a while. Call periodically.
func PruneTracker(maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge)
	trackerMu.Lock()
	for mint, mt := range tracker {
		if len(mt.events) == 0 || mt.events[len(mt.events)-1].ts.Before(cutoff) {
			delete(tracker, mint)
			continue
		}
		// trim old events within the mint
		i := 0
		for i < len(mt.events) && mt.events[i].ts.Before(cutoff) {
			i++
		}
		if i > 0 {
			mt.events = mt.events[i:]
		}
	}
	trackerMu.Unlock()
}

// ResetTracker clears all tracked data (for tests).
func ResetTracker() {
	trackerMu.Lock()
	tracker = map[string]*mintTracker{}
	trackerMu.Unlock()
}
