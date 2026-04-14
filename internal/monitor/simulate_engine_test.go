package monitor

import (
	"testing"
	"time"

	"rlangga/internal/bot"
	"rlangga/internal/config"
	"rlangga/internal/store"
	"rlangga/internal/testutil"
)

func TestMonitorPositionWithBot_SimulateEngineFollowsRealPath(t *testing.T) {
	testutil.UseMiniredis(t)
	t.Setenv("RPC_STUB", "1")
	t.Setenv("SIMULATE_ENGINE", "1")
	t.Setenv("PUMPPORTAL_URL", "")
	t.Setenv("PUMPAPI_URL", "")
	t.Setenv("QUOTE_INTERVAL_MS", "20")
	t.Setenv("MAX_HOLD", "1")
	t.Setenv("MIN_HOLD", "1")
	t.Setenv("GRACE_SECONDS", "0")
	t.Setenv("PANIC_SL", "8")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { config.C = nil })

	b := bot.FromConfig(cfg)
	done := make(chan struct{})
	go func() {
		MonitorPositionWithBot("So11111111111111111111111111111111111111112", 0.1, b, nil, nil)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("monitor did not exit")
	}

	trades, err := store.LoadRecent(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	tr := trades[0]
	if tr.ExitReason == "" {
		t.Fatal("expected exit reason to be set")
	}
	if tr.BuySOL != 0.1 {
		t.Fatalf("expected BuySOL=0.1, got %v", tr.BuySOL)
	}
}
