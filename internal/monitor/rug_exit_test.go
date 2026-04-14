package monitor

import (
	"testing"
	"time"

	"rlangga/internal/bot"
	"rlangga/internal/config"
	"rlangga/internal/exit"
	"rlangga/internal/pumpws"
	"rlangga/internal/store"
	"rlangga/internal/testutil"
)

func TestMonitorPositionWithBot_ExitOnRugRemoveEvent(t *testing.T) {
	testutil.UseMiniredis(t)
	t.Setenv("RPC_STUB", "1")
	t.Setenv("SIMULATE_ENGINE", "1")
	t.Setenv("PUMPPORTAL_URL", "")
	t.Setenv("PUMPAPI_URL", "")
	t.Setenv("QUOTE_INTERVAL_MS", "200")
	t.Setenv("MAX_HOLD", "12")
	t.Setenv("MIN_HOLD", "4")
	t.Setenv("GRACE_SECONDS", "1")
	t.Setenv("PANIC_SL", "99")
	t.Setenv("SL_PERCENT", "4")
	t.Setenv("TP_PERCENT", "999")
	t.Setenv("MOMENTUM_DROP", "2")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { config.C = nil })

	mint := "E9qgYkgok8aCFXWuvQiYM8krmDbeTuARLgrBjThWpump"
	b := bot.FromConfig(cfg)

	done := make(chan struct{})
	go func() {
		MonitorPositionWithBot(mint, 0.1, b, nil, nil)
		close(done)
	}()

	// Give monitor time to subscribe.
	time.Sleep(50 * time.Millisecond)
	pumpws.PublishStreamEvent(pumpws.StreamEvent{
		Mint:   mint,
		TxType: "remove",
		Pool:   "pump-amm",
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("monitor did not exit on rug remove event")
	}

	trades, err := store.LoadRecent(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if trades[0].ExitReason != exit.ExitRugRemove {
		t.Fatalf("expected exit reason %q, got %q", exit.ExitRugRemove, trades[0].ExitReason)
	}
}
