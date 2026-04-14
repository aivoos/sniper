package quote

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"rlangga/internal/config"
)

func TestRaceQuote_FasterProviderWins(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/quote" {
			http.NotFound(w, r)
			return
		}
		time.Sleep(80 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]float64{"sol": 0.1})
	}))
	fast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/quote" {
			http.NotFound(w, r)
			return
		}
		time.Sleep(5 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]float64{"sol": 0.2})
	}))
	t.Cleanup(slow.Close)
	t.Cleanup(fast.Close)

	t.Setenv("PUMPPORTAL_URL", slow.URL)
	t.Setenv("PUMPAPI_URL", fast.URL)
	t.Setenv("TIMEOUT_MS", "3000")
	t.Setenv("RPC_STUB", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}

	sol, _, preferPortal, ok := RaceQuote("mint")
	if !ok {
		t.Fatal("expected ok")
	}
	if preferPortal {
		t.Fatal("API should be faster")
	}
	if sol != 0.2 {
		t.Fatalf("sol want 0.2 got %v", sol)
	}
}

func TestRaceQuote_OnlyPortalConfigured(t *testing.T) {
	h := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]float64{"sol": 0.5})
	}))
	t.Cleanup(h.Close)
	t.Setenv("PUMPPORTAL_URL", h.URL)
	t.Setenv("PUMPAPI_URL", "")
	t.Setenv("RPC_STUB", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	_, _, preferPortal, ok := RaceQuote("m")
	if !ok || !preferPortal {
		t.Fatalf("ok=%v preferPortal=%v", ok, preferPortal)
	}
}
