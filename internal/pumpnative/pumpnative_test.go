package pumpnative

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"rlangga/internal/config"
)

func TestPortalTradeURL_WithAPIKey(t *testing.T) {
	cfg := &config.Config{
		PumpPortalURL:    "https://example.com/api/trade",
		PumpPortalAPIKey: "k&x",
	}
	u := PortalTradeURL(cfg)
	if u == "" {
		t.Fatal("empty url")
	}
}

func TestShouldUsePortalNative(t *testing.T) {
	if ShouldUsePortalNative(nil) {
		t.Fatal("nil")
	}
	if ShouldUsePortalNative(&config.Config{PumpNative: true}) {
		t.Fatal("no key")
	}
	if !ShouldUsePortalNative(&config.Config{PumpNative: true, PumpPortalAPIKey: "a"}) {
		t.Fatal("with key")
	}
}

func TestShouldUseAPINative(t *testing.T) {
	if ShouldUseAPINative(nil) {
		t.Fatal("nil")
	}
	if ShouldUseAPINative(&config.Config{PumpNative: true}) {
		t.Fatal("no identity")
	}
	cfg := &config.Config{PumpNative: true, WalletPublicKey: "So11111111111111111111111111111111111111112"}
	if !ShouldUseAPINative(cfg) {
		t.Fatal("default api base")
	}
	cfg.PumpAPIURL = "https://other.example/v1"
	if ShouldUseAPINative(cfg) {
		t.Fatal("non-pumpapi host")
	}
}

func TestApplyPumpAPIIdentity_Errors(t *testing.T) {
	if err := applyPumpAPIIdentity(nil, map[string]interface{}{}); err == nil {
		t.Fatal("nil cfg")
	}
	if err := applyPumpAPIIdentity(&config.Config{}, map[string]interface{}{}); err == nil {
		t.Fatal("no keys")
	}
}

func TestParseTxSignatureJSON(t *testing.T) {
	_, err := ParseTxSignatureJSON([]byte(`{"error":"bad"}`))
	if err == nil {
		t.Fatal("error field")
	}
	sig, err := ParseTxSignatureJSON([]byte(`{"signature":"abc"}`))
	if err != nil || sig != "abc" {
		t.Fatalf("sig=%q err=%v", sig, err)
	}
	sig, err = ParseTxSignatureJSON([]byte(`{"signatures":["z"]}`))
	if err != nil || sig != "z" {
		t.Fatalf("sig=%q err=%v", sig, err)
	}
}

func TestPostJSON_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"signature": "sig1"})
	}))
	t.Cleanup(srv.Close)
	sig, err := postJSON(srv.URL, map[string]interface{}{"a": 1}, 1000)
	if err != nil || sig != "sig1" {
		t.Fatalf("sig=%q err=%v", sig, err)
	}
}

func TestPortalBuy_MockServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"signature": "pbuy"})
	}))
	t.Cleanup(srv.Close)
	cfg := &config.Config{
		PumpPortalURL: srv.URL,
		PumpSlippage:  1,
		TimeoutMS:     1000,
	}
	sig, err := PortalBuy(cfg, "So11111111111111111111111111111111111111112", 0.1)
	if err != nil || sig != "pbuy" {
		t.Fatalf("sig=%q err=%v", sig, err)
	}
}

func TestAPIBuy_MockServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"signature": "abuy"})
	}))
	t.Cleanup(srv.Close)
	cfg := &config.Config{
		PumpAPIURL:      srv.URL,
		PumpAPISlippage: 1,
		WalletPublicKey: "So11111111111111111111111111111111111111112",
		TimeoutMS:       1000,
	}
	sig, err := APIBuy(cfg, "So11111111111111111111111111111111111111112", 0.1)
	if err != nil || sig != "abuy" {
		t.Fatalf("sig=%q err=%v", sig, err)
	}
}

func TestPostJSON_NonJSONBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte{0, 1, 2})
	}))
	t.Cleanup(srv.Close)
	_, err := postJSON(srv.URL, map[string]interface{}{}, 1000)
	if err != ErrNonJSONResponse {
		t.Fatalf("got %v", err)
	}
}
