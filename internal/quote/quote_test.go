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
	t.Setenv("RPC_STUB", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}

	q := GetSellQuote("mint")
	if q != 0.12 {
		t.Fatalf("quote: %v", q)
	}
}

func TestGetSellQuote_AmountField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]float64{"amount": 0.2})
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PUMPPORTAL_URL", srv.URL+"/")
	t.Setenv("RPC_STUB", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if GetSellQuote("m") != 0.2 {
		t.Fatal("amount fallback")
	}
}

func TestPostQuote_NoSolField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"foo": "bar"})
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PUMPPORTAL_URL", srv.URL)
	t.Setenv("RPC_STUB", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if GetSellQuote("m") != 0 {
		t.Fatal("expected 0 when no sol")
	}
}

func TestPostQuote_BadHTTPStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PUMPPORTAL_URL", srv.URL)
	t.Setenv("RPC_STUB", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if GetSellQuote("m") != 0 {
		t.Fatal("expected 0 on bad status")
	}
}

func TestGetSellQuote_QuoteField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]float64{"quote": 0.3})
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PUMPPORTAL_URL", srv.URL)
	t.Setenv("RPC_STUB", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if GetSellQuote("m") != 0.3 {
		t.Fatal("quote fallback")
	}
}
