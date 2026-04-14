package guard

import (
	"time"

	"rlangga/internal/config"
)

// ActiveHourWindowContains reports whether the local hour of t in loc falls in the trading window.
// When startH < 0 or endH < 0, returns true (window disabled — caller treats as always allowed).
// Same calendar day: startH <= endH → inclusive [startH, endH]. Overnight wrap: startH > endH → hour >= startH || hour <= endH.
func ActiveHourWindowContains(loc *time.Location, startH, endH int, t time.Time) bool {
	if startH < 0 || endH < 0 {
		return true
	}
	h := t.In(loc).Hour()
	if startH <= endH {
		return h >= startH && h <= endH
	}
	return h >= startH || h <= endH
}

// IsWithinActiveTradingHours is true when active-hours governance is off or current time is inside the window (BUY gate only).
func IsWithinActiveTradingHours() bool {
	cfg := config.C
	if cfg == nil || cfg.ActiveStartHour < 0 || cfg.ActiveEndHour < 0 {
		return true
	}
	loc := time.UTC
	if cfg.TZ != "" {
		if l, err := time.LoadLocation(cfg.TZ); err == nil {
			loc = l
		}
	}
	return ActiveHourWindowContains(loc, cfg.ActiveStartHour, cfg.ActiveEndHour, time.Now())
}
