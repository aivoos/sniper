package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"rlangga/internal/config"
)

// Token is a wallet holding for a mint (amount in UI units as float for PR-001 stub).
type Token struct {
	Mint   string
	Amount float64
}

// WaitTxConfirmed polls RPC until the signature reaches a confirmed state or times out.
func WaitTxConfirmed(sig string) bool {
	if sig == "" {
		return false
	}
	if config.C != nil && config.C.RPCStub {
		return true
	}
	for i := 0; i < 10; i++ {
		status := getTxStatus(sig)
		if status == "confirmed" {
			return true
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false
}

func getTxStatus(sig string) string {
	if config.C == nil || config.C.RPCURL == "" {
		return "unknown"
	}
	// JSON-RPC: getSignatureStatuses
	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getSignatureStatuses",
		"params":  [][]string{{sig}},
	}
	raw, _ := json.Marshal(body)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.C.TimeoutMS)*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, config.C.RPCURL, bytes.NewReader(raw))
	if err != nil {
		return "unknown"
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "unknown"
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var out struct {
		Result struct {
			Value []struct {
				ConfirmationStatus *string `json:"confirmationStatus"`
			} `json:"value"`
		} `json:"result"`
	}
	if json.Unmarshal(b, &out) != nil {
		return "unknown"
	}
	if len(out.Result.Value) == 0 || out.Result.Value[0].ConfirmationStatus == nil {
		return "unknown"
	}
	st := *out.Result.Value[0].ConfirmationStatus
	if st == "finalized" || st == "confirmed" {
		return "confirmed"
	}
	return st
}

// GetWalletTokens returns SPL token accounts for the trading wallet (stub: empty until wired).
func GetWalletTokens() []Token {
	// TODO: RPC getTokenAccountsByOwner + parse; PR-001 returns empty so recovery is no-op without keys.
	return []Token{}
}
