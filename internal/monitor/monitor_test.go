package monitor_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"rlangga/internal/config"
	"rlangga/internal/monitor"
	"rlangga/internal/testutil"
)

func TestMonitorPosition_PanicExitFirstQuote(t *testing.T) {
	testutil.UseMiniredis(t)
	t.Setenv("RPC_STUB", "1")
	t.Setenv("QUOTE_INTERVAL_MS", "20")
	t.Setenv("PANIC_SL", "8")
	t.Setenv("GRACE_SECONDS", "2")
	t.Setenv("MIN_HOLD", "5")
	t.Setenv("MAX_HOLD", "15")

	portal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/quote":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]float64{"sol": 0})
		case "/sell":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"signature": "sell-sig"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(portal.Close)

	t.Setenv("PUMPPORTAL_URL", portal.URL)
	t.Setenv("PUMPAPI_URL", portal.URL)

	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		monitor.MonitorPosition("So11111111111111111111111111111111111111112", 0.1)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("monitor did not exit")
	}
}

func TestMonitorPosition_ZeroIntervalUsesDefault(t *testing.T) {
	testutil.UseMiniredis(t)
	t.Setenv("RPC_STUB", "1")
	t.Setenv("QUOTE_INTERVAL_MS", "1")
	t.Setenv("PANIC_SL", "8")
	t.Setenv("PUMPPORTAL_URL", "")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	prevQ := config.C.QuoteIntervalMS
	prevP := config.C.PumpPortalURL
	prevA := config.C.PumpAPIURL
	t.Cleanup(func() {
		if config.C != nil {
			config.C.QuoteIntervalMS = prevQ
			config.C.PumpPortalURL = prevP
			config.C.PumpAPIURL = prevA
		}
	})
	config.C.QuoteIntervalMS = 0

	portal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/quote" {
			_ = json.NewEncoder(w).Encode(map[string]float64{"sol": 0})
		}
		if r.URL.Path == "/sell" {
			_ = json.NewEncoder(w).Encode(map[string]string{"signature": "s"})
		}
	}))
	t.Cleanup(portal.Close)
	config.C.PumpPortalURL = portal.URL
	config.C.PumpAPIURL = portal.URL

	done := make(chan struct{})
	go func() {
		monitor.MonitorPosition("So11111111111111111111111111111111111111112", 0.1)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(4 * time.Second):
		t.Fatal("timeout")
	}
}

func TestMonitorPosition_NilConfig(t *testing.T) {
	old := config.C
	config.C = nil
	t.Cleanup(func() { config.C = old })
	done := make(chan struct{})
	go func() {
		monitor.MonitorPosition("m", 0.1)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("expected immediate return when config nil")
	}
}
