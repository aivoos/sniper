package executor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"rlangga/internal/config"
)

func TestBuyAndValidate_PortalPrimary(t *testing.T) {
	portal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/buy" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"signature": "portal-sig"})
	}))
	t.Cleanup(portal.Close)
	t.Setenv("PUMPPORTAL_URL", portal.URL)
	t.Setenv("PUMPAPI_URL", "")
	t.Setenv("RPC_STUB", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if !BuyAndValidate("So11111111111111111111111111111111111111112") {
		t.Fatal("expected portal path")
	}
}

func TestBuyAndValidate_FallbackAndConfirm(t *testing.T) {
	pumpFail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	pumpOK := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/buy" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"signature": "sig-from-api"})
	}))
	rpcSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"result": map[string]interface{}{
				"value": []interface{}{
					map[string]interface{}{"confirmationStatus": "confirmed"},
				},
			},
		})
	}))
	t.Cleanup(pumpFail.Close)
	t.Cleanup(pumpOK.Close)
	t.Cleanup(rpcSrv.Close)

	t.Setenv("PUMPPORTAL_URL", pumpFail.URL)
	t.Setenv("PUMPAPI_URL", pumpOK.URL)
	t.Setenv("RPC_URL", rpcSrv.URL)
	t.Setenv("RPC_STUB", "0")
	t.Setenv("PUMP_WALLET_PUBLIC_KEY", "So11111111111111111111111111111111111111112")
	t.Setenv("TIMEOUT_MS", "3000")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}

	if !BuyAndValidate("So11111111111111111111111111111111111111112") {
		t.Fatal("expected buy success via API fallback + RPC confirm")
	}
}

func TestBuyAndValidate_NoURLs(t *testing.T) {
	t.Setenv("PUMPPORTAL_URL", "")
	t.Setenv("PUMPAPI_URL", "")
	t.Setenv("RPC_STUB", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if BuyAndValidate("mint") {
		t.Fatal("expected false when pump not configured")
	}
}

func TestBuyAndValidate_SimulateEngine(t *testing.T) {
	t.Setenv("PUMPPORTAL_URL", "")
	t.Setenv("PUMPAPI_URL", "")
	t.Setenv("RPC_STUB", "1")
	t.Setenv("SIMULATE_ENGINE", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if !BuyAndValidate("mintX") {
		t.Fatal("expected true for simulate engine without HTTP")
	}
}

func TestSafeSellWithValidation_SimulateEngine(t *testing.T) {
	t.Setenv("PUMPPORTAL_URL", "")
	t.Setenv("PUMPAPI_URL", "")
	t.Setenv("RPC_STUB", "1")
	t.Setenv("SIMULATE_ENGINE", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if !SafeSellWithValidation("mintY") {
		t.Fatal("expected true for simulate engine without HTTP")
	}
}

func TestBuyAndValidate_InvalidBuyJSON(t *testing.T) {
	portal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/buy" {
			_, _ = w.Write([]byte(`{`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(portal.Close)
	t.Setenv("PUMPPORTAL_URL", portal.URL)
	t.Setenv("PUMPAPI_URL", "")
	t.Setenv("RPC_STUB", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if BuyAndValidate("m") {
		t.Fatal("expected false on bad JSON")
	}
}

// TestSimulation_QuoteRacePreferFastBuy mensimulasikan: quote API lebih cepat dari Portal,
// sehingga RaceQuote memilih API dan BuyAndValidate memanggil /buy ke API dulu (bukan Portal).
func TestSimulation_QuoteRacePreferFastBuy(t *testing.T) {
	var portalBuyHits, apiBuyHits int
	portal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/quote":
			time.Sleep(50 * time.Millisecond)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]float64{"sol": 0.1})
		case "/buy":
			portalBuyHits++
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"signature": "portal-buy"})
		default:
			http.NotFound(w, r)
		}
	}))
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/quote":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]float64{"sol": 0.2})
		case "/buy":
			apiBuyHits++
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"signature": "api-buy"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(portal.Close)
	t.Cleanup(api.Close)

	t.Setenv("PUMPPORTAL_URL", portal.URL)
	t.Setenv("PUMPAPI_URL", api.URL)
	t.Setenv("TIMEOUT_MS", "3000")
	t.Setenv("RPC_STUB", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}

	if !BuyAndValidate("So11111111111111111111111111111111111111112") {
		t.Fatal("expected buy ok")
	}
	if apiBuyHits != 1 || portalBuyHits != 0 {
		t.Fatalf("smart order: api buy hits=%d portal buy hits=%d (want api first only)", apiBuyHits, portalBuyHits)
	}
}

// TestBuyAndValidate_NoSecondBuyIfPortalSubmitted: portal mengembalikan signature tetapi RPC tidak pernah
// "confirmed" — PumpAPI /buy tidak boleh dipanggil (hindari double BUY).
func TestBuyAndValidate_NoSecondBuyIfPortalSubmitted(t *testing.T) {
	if testing.Short() {
		t.Skip("poll RPC panjang")
	}
	var portalHits, apiHits int
	portal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/buy":
			portalHits++
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"signature": "pend"})
		case "/quote":
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/buy":
			apiHits++
			t.Fatal("PumpAPI /buy must not be called after Portal submitted a signature")
		case "/quote":
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	rpcSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"result": map[string]interface{}{
				"value": []interface{}{
					map[string]interface{}{"confirmationStatus": "processed"},
				},
			},
		})
	}))
	t.Cleanup(portal.Close)
	t.Cleanup(api.Close)
	t.Cleanup(rpcSrv.Close)
	t.Setenv("PUMPPORTAL_URL", portal.URL)
	t.Setenv("PUMPAPI_URL", api.URL)
	t.Setenv("RPC_URL", rpcSrv.URL)
	t.Setenv("RPC_STUB", "0")
	t.Setenv("PUMP_WALLET_PUBLIC_KEY", "So11111111111111111111111111111111111111112")
	t.Setenv("TIMEOUT_MS", "800")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if BuyAndValidate("mint") {
		t.Fatal("expected false when confirmation never succeeds")
	}
	if portalHits != 1 {
		t.Fatalf("portalHits=%d", portalHits)
	}
	if apiHits != 0 {
		t.Fatalf("apiHits=%d", apiHits)
	}
}

func TestSafeSellWithValidation_Success(t *testing.T) {
	portal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sell" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"signature": "sell-sig"})
	}))
	t.Cleanup(portal.Close)
	t.Setenv("PUMPPORTAL_URL", portal.URL)
	t.Setenv("RPC_STUB", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	SafeSellWithValidation("mintZ")
}
