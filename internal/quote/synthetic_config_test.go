package quote

import (
	"testing"

	"rlangga/internal/config"
)

func TestSyntheticSellQuoteForEngine_UsesConfigTuning(t *testing.T) {
	t.Setenv("RPC_STUB", "1")
	t.Setenv("SIMULATE_SYNTH_AMPLITUDE_PCT", "20")
	t.Setenv("SIMULATE_SYNTH_PERIOD_SEC", "10")
	t.Setenv("SIMULATE_SYNTH_DRIFT_PCT", "0.5")
	t.Cleanup(func() { config.C = nil })
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SimulateSynthAmplitudePct != 20 || cfg.SimulateSynthPeriodSec != 10 {
		t.Fatal(cfg)
	}
	// Deterministic: same mint + elapsed → same quote
	q := SyntheticSellQuoteForEngine("mintX", 1.0, 3)
	q2 := SyntheticSellQuoteForEngine("mintX", 1.0, 3)
	if q != q2 || q <= 0 {
		t.Fatalf("got %v %v", q, q2)
	}
}

func TestSyntheticSellQuoteForEngine_NoConfigUsesDefaults(t *testing.T) {
	config.C = nil
	t.Cleanup(func() { config.C = nil })
	a := SyntheticSellQuoteForEngine("z", 1.0, 4)
	if a <= 0 {
		t.Fatal(a)
	}
}
