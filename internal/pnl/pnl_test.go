package pnl

import "testing"

func TestCalcPnL(t *testing.T) {
	p := CalcPnL(0.1, 0.11)
	if p < 9.9 || p > 10.1 {
		t.Fatalf("expected ~10%% got %v", p)
	}
	if CalcPnL(0, 1) != 0 {
		t.Fatal("zero buy should return 0")
	}
}

func TestCalcPnL_Deterministic(t *testing.T) {
	buy, sell := 0.1, 0.107
	a := CalcPnL(buy, sell)
	b := CalcPnL(buy, sell)
	if a != b {
		t.Fatal("non-deterministic")
	}
}
