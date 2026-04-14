package guard

import (
	"testing"
	"time"
)

func TestActiveHourWindowContains_SameDay(t *testing.T) {
	loc := time.UTC
	// 09:00–17:00 inclusive
	if !ActiveHourWindowContains(loc, 9, 17, time.Date(2026, 4, 13, 9, 0, 0, 0, loc)) {
		t.Fatal("9:00 in")
	}
	if !ActiveHourWindowContains(loc, 9, 17, time.Date(2026, 4, 13, 17, 0, 0, 0, loc)) {
		t.Fatal("17:00 in")
	}
	if ActiveHourWindowContains(loc, 9, 17, time.Date(2026, 4, 13, 8, 59, 0, 0, loc)) {
		t.Fatal("08:59 out")
	}
	if ActiveHourWindowContains(loc, 9, 17, time.Date(2026, 4, 13, 18, 0, 0, 0, loc)) {
		t.Fatal("18:00 out")
	}
}

func TestActiveHourWindowContains_WrapMidnight(t *testing.T) {
	loc := time.UTC
	// 20:00–02:00 overnight
	if !ActiveHourWindowContains(loc, 20, 2, time.Date(2026, 4, 13, 21, 0, 0, 0, loc)) {
		t.Fatal("21:00 in")
	}
	if !ActiveHourWindowContains(loc, 20, 2, time.Date(2026, 4, 13, 0, 30, 0, 0, loc)) {
		t.Fatal("00:30 in")
	}
	if !ActiveHourWindowContains(loc, 20, 2, time.Date(2026, 4, 13, 2, 0, 0, 0, loc)) {
		t.Fatal("02:00 in")
	}
	if ActiveHourWindowContains(loc, 20, 2, time.Date(2026, 4, 13, 19, 0, 0, 0, loc)) {
		t.Fatal("19:00 out")
	}
	if ActiveHourWindowContains(loc, 20, 2, time.Date(2026, 4, 13, 3, 0, 0, 0, loc)) {
		t.Fatal("03:00 out")
	}
}

func TestActiveHourWindowContains_DisabledNegative(t *testing.T) {
	loc := time.UTC
	if !ActiveHourWindowContains(loc, -1, -1, time.Date(2026, 4, 13, 12, 0, 0, 0, loc)) {
		t.Fatal("disabled")
	}
}
