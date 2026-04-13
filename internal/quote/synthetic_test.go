package quote

import (
	"testing"

	"rlangga/internal/config"
)

func TestSyntheticSellQuoteForEngine_Deterministic(t *testing.T) {
	t.Cleanup(func() { config.C = nil })
	a := SyntheticSellQuoteForEngine("mintA", 1.0, 5)
	b := SyntheticSellQuoteForEngine("mintA", 1.0, 5)
	if a != b {
		t.Fatalf("same mint+elapsed should match: %v vs %v", a, b)
	}
	if a <= 0 {
		t.Fatal("expected positive sell quote")
	}
}
