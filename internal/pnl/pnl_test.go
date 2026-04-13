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

func TestApplyFees(t *testing.T) {
	buyCost, sellProceeds := ApplyFees(1, 1, 0.25, 0.25)
	// 1 * (1+0.0025) = 1.0025
	if buyCost < 1.0024 || buyCost > 1.0026 {
		t.Fatalf("buyCost %v", buyCost)
	}
	// 1 * (1-0.0025) = 0.9975
	if sellProceeds < 0.9974 || sellProceeds > 0.9976 {
		t.Fatalf("sellProceeds %v", sellProceeds)
	}
}
