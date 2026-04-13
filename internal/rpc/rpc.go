package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"rlangga/internal/config"
)

// Token is an SPL token holding (UI amount) for recovery / reporting.
type Token struct {
	Mint   string
	Amount float64
}

var preferredRPCIdx atomic.Uint32

// rpcEndpoints mengembalikan daftar RPC (RPC_URLS) atau satu RPC_URL.
func rpcEndpoints() []string {
	if config.C == nil {
		return nil
	}
	if len(config.C.RPCURLs) > 0 {
		return config.C.RPCURLs
	}
	if config.C.RPCURL != "" {
		return []string{config.C.RPCURL}
	}
	return nil
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

// ConfirmTransaction mem-poll lebih lama dari WaitTxConfirmed. Dipakai setelah BUY/SELL ketika
// transaksi sudah punya signature agar tidak memicu eksekusi kedua ke provider lain (hindari double spend).
func ConfirmTransaction(sig string) bool {
	if WaitTxConfirmed(sig) {
		return true
	}
	if sig == "" {
		return false
	}
	if config.C != nil && config.C.RPCStub {
		return true
	}
	for i := 0; i < 24; i++ {
		if getTxStatus(sig) == "confirmed" {
			return true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

func getTxStatus(sig string) string {
	endpoints := rpcEndpoints()
	if len(endpoints) == 0 {
		return "unknown"
	}
	n := len(endpoints)
	start := int(preferredRPCIdx.Load() % uint32(n))
	for j := 0; j < n; j++ {
		idx := (start + j) % n
		url := endpoints[idx]
		st, ok := getTxStatusAt(url, sig)
		if !ok {
			// gagal transport / parse / JSON-RPC error → coba endpoint berikutnya
			continue
		}
		preferredRPCIdx.Store(uint32(idx))
		if st == "confirmed" {
			return "confirmed"
		}
		return st
	}
	return "unknown"
}

// getTxStatusAt memanggil getSignatureStatuses ke satu URL. ok=false jika perlu failover.
func getTxStatusAt(rpcURL, sig string) (status string, ok bool) {
	if config.C == nil || rpcURL == "" {
		return "unknown", false
	}
	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getSignatureStatuses",
		"params":  [][]string{{sig}},
	}
	raw, _ := json.Marshal(body)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.C.TimeoutMS)*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(raw))
	if err != nil {
		return "unknown", false
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "unknown", false
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "unknown", false
	}
	var envelope struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Result struct {
			Value []struct {
				ConfirmationStatus *string `json:"confirmationStatus"`
			} `json:"value"`
		} `json:"result"`
	}
	if json.Unmarshal(b, &envelope) != nil {
		return "unknown", false
	}
	if envelope.Error != nil {
		return "unknown", false
	}
	if len(envelope.Result.Value) == 0 || envelope.Result.Value[0].ConfirmationStatus == nil {
		return "unknown", true
	}
	st := *envelope.Result.Value[0].ConfirmationStatus
	if st == "finalized" || st == "confirmed" {
		return "confirmed", true
	}
	return st, true
}

// WalletTokensHook is set by tests to simulate holdings; nil uses RPC getTokenAccountsByOwner.
var WalletTokensHook func() []Token

// GetWalletTokens returns SPL token accounts for PUMP_WALLET_PUBLIC_KEY via RPC (legacy + Token-2022).
func GetWalletTokens() []Token {
	if WalletTokensHook != nil {
		return WalletTokensHook()
	}
	toks := getWalletTokensFromRPC()
	if toks == nil {
		return []Token{}
	}
	return toks
}
