package pumpnative

import (
	"testing"

	"rlangga/internal/config"
)

func TestParseTxSignatureJSON(t *testing.T) {
	sig, err := ParseTxSignatureJSON([]byte(`{"signature":"abc123"}`))
	if err != nil || sig != "abc123" {
		t.Fatalf("got %q %v", sig, err)
	}
	sig, err = ParseTxSignatureJSON([]byte(`{"signatures":["xyz"]}`))
	if err != nil || sig != "xyz" {
		t.Fatalf("got %q %v", sig, err)
	}
}

func TestPortalTradeURL_WithKey(t *testing.T) {
	cfg := &config.Config{
		PumpPortalURL:    "https://pumpportal.fun/api/trade",
		PumpPortalAPIKey: "k",
	}
	u := PortalTradeURL(cfg)
	if u != "https://pumpportal.fun/api/trade?api-key=k" {
		t.Fatalf("got %s", u)
	}
}

func TestShouldUseAPINative(t *testing.T) {
	if ShouldUseAPINative(&config.Config{PumpNative: true, WalletPublicKey: "x", PumpAPIURL: ""}) != true {
		t.Fatal("empty API url + wallet")
	}
	if ShouldUseAPINative(&config.Config{PumpNative: true, WalletPublicKey: "x", PumpAPIURL: "https://api.pumpapi.io"}) != true {
		t.Fatal("pumpapi host")
	}
	if ShouldUseAPINative(&config.Config{PumpNative: true, WalletPublicKey: "x", PumpAPIURL: "http://localhost/proxy"}) != false {
		t.Fatal("legacy gateway should not use native")
	}
	if ShouldUseAPINative(&config.Config{PumpNative: true, PumpPrivateKey: "secretbase58", PumpAPIURL: ""}) != true {
		t.Fatal("private key alone should enable PumpAPI native")
	}
}
