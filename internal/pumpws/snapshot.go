package pumpws

import "time"

// ActivitySnapshot is tracker state at BUY time (for analytics on saved trades).
type ActivitySnapshot struct {
	BuySellRatio float64
	McapRising   bool
}

// SnapshotAtBuy returns activity state for mint (nil if no tracker data).
func SnapshotAtBuy(mint string, window time.Duration) *ActivitySnapshot {
	act := GetMintActivity(mint, window)
	if act == nil {
		return nil
	}
	return &ActivitySnapshot{
		BuySellRatio: act.BuySellRatio(),
		McapRising:   act.McapRising(),
	}
}
