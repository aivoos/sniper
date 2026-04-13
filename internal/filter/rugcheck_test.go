package filter

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"rlangga/internal/config"
)

// mintBytes82 builds minimal valid SPL mint: no freeze (opt 0), some supply, initialized.
func mintBytes82(freezeOpt uint32, mintAuthOpt uint32, supply uint64) []byte {
	b := make([]byte, 82)
	binary.LittleEndian.PutUint32(b[0:4], mintAuthOpt)
	binary.LittleEndian.PutUint64(b[36:44], supply)
	b[44] = 6
	b[45] = 1
	binary.LittleEndian.PutUint32(b[46:50], freezeOpt)
	return b
}

func TestAllowMint_GoodMint(t *testing.T) {
	raw := mintBytes82(0, 0, 1_000_000_000_000)
	b64 := base64.StdEncoding.EncodeToString(raw)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		if req.Method == "getAccountInfo" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]interface{}{
					"context": map[string]int{"slot": 1},
					"value": map[string]interface{}{
						"owner": programTokenLegacy,
						"data":  []interface{}{b64, "base64"},
					},
				},
			})
			return
		}
		if req.Method == "getTokenSupply" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]interface{}{
					"value": map[string]interface{}{
						"amount": "1000000000000",
					},
				},
			})
			return
		}
		if req.Method == "getTokenLargestAccounts" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]interface{}{
					"value": []map[string]interface{}{
						{"amount": "10000000000"},
					},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	t.Setenv("FILTER_ANTI_RUG", "1")
	t.Setenv("FILTER_REJECT_FREEZE_AUTHORITY", "1")
	t.Setenv("FILTER_REJECT_MINT_AUTHORITY", "0")
	t.Setenv("FILTER_MAX_TOP_HOLDER_PCT", "90")
	t.Setenv("FILTER_RPC_FAIL_OPEN", "0")
	t.Setenv("RPC_URL", srv.URL)
	t.Setenv("RPC_STUB", "0")
	t.Setenv("ENABLE_TRADING", "false")
	t.Setenv("TIMEOUT_MS", "3000")
	unset := []string{"RPC_URLS", "PAPER_TRADE", "HELIUS_API_KEY"}
	for _, k := range unset {
		t.Setenv(k, "")
	}
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}

	ok, reason := AllowMint(context.Background(), "So11111111111111111111111111111111111111112")
	if !ok {
		t.Fatalf("expected pass: %s", reason)
	}
}

func TestAllowMint_FreezeAuthority(t *testing.T) {
	raw := mintBytes82(1, 0, 1_000_000_000_000)
	b64 := base64.StdEncoding.EncodeToString(raw)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]interface{}{
				"value": map[string]interface{}{
					"owner": programTokenLegacy,
					"data":  []interface{}{b64, "base64"},
				},
			},
		})
	}))
	t.Cleanup(srv.Close)

	t.Setenv("FILTER_ANTI_RUG", "1")
	t.Setenv("FILTER_REJECT_FREEZE_AUTHORITY", "1")
	t.Setenv("RPC_URL", srv.URL)
	t.Setenv("RPC_STUB", "0")
	t.Setenv("ENABLE_TRADING", "false")
	t.Setenv("TIMEOUT_MS", "3000")
	t.Setenv("RPC_URLS", "")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	ok, reason := AllowMint(context.Background(), "So11111111111111111111111111111111111111112")
	if ok || reason == "" {
		t.Fatalf("expected reject freeze, ok=%v reason=%q", ok, reason)
	}
}

func TestAllowMint_Disabled(t *testing.T) {
	t.Setenv("FILTER_ANTI_RUG", "0")
	t.Setenv("RPC_STUB", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	ok, _ := AllowMint(context.Background(), "any")
	if !ok {
		t.Fatal("expected pass when filter off")
	}
}
