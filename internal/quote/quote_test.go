package quote

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"rlangga/internal/config"
)

func TestGetSellQuote_PortalThenAPI(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/quote" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]float64{"sol": 0.12})
	}))
	t.Cleanup(bad.Close)
	t.Cleanup(good.Close)

	t.Setenv("PUMPPORTAL_URL", bad.URL)
	t.Setenv("PUMPAPI_URL", good.URL)
	t.Setenv("TIMEOUT_MS", "2000")
	config.Load()

	q := GetSellQuote("mint")
	if q != 0.12 {
		t.Fatalf("quote: %v", q)
	}
}
