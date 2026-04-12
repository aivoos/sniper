package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"rlangga/internal/config"
	"rlangga/internal/log"
	"rlangga/internal/rpc"
)

var errNotConfigured = errors.New("executor: pump URL not configured")

// BuyAndValidate tries PumpPortal then PumpAPI; confirms via RPC.
func BuyAndValidate(mint string) bool {
	sig, err := pumpPortalBuy(mint)
	if err != nil {
		sig, err = pumpApiBuy(mint)
		if err != nil {
			return false
		}
	}
	return rpc.WaitTxConfirmed(sig)
}

// SafeSellWithValidation retries sell with RPC confirmation.
func SafeSellWithValidation(mint string) bool {
	for i := 0; i < 5; i++ {
		sig, err := sell(mint)
		if err == nil && rpc.WaitTxConfirmed(sig) {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	log.Error("SELL FAILED: " + mint)
	return false
}

func pumpPortalBuy(mint string) (string, error) {
	if config.C == nil || config.C.PumpPortalURL == "" || config.C.PumpPortalURL == "xxx" {
		return "", errNotConfigured
	}
	return postMint(config.C.PumpPortalURL, mint)
}

func pumpApiBuy(mint string) (string, error) {
	if config.C == nil || config.C.PumpAPIURL == "" || config.C.PumpAPIURL == "xxx" {
		return "", errNotConfigured
	}
	return postMint(config.C.PumpAPIURL, mint)
}

func sell(mint string) (string, error) {
	// Same endpoints pattern as buy until product API is fixed (swap path /sell).
	if config.C == nil || config.C.PumpPortalURL == "" || config.C.PumpPortalURL == "xxx" {
		return "", errNotConfigured
	}
	u := config.C.PumpPortalURL + "/sell"
	return postRaw(u, mint)
}

func postMint(baseURL, mint string) (string, error) {
	u := baseURL
	if u[len(u)-1] == '/' {
		u = u[:len(u)-1]
	}
	return postRaw(u+"/buy", mint)
}

func postRaw(url, mint string) (string, error) {
	body := map[string]string{"mint": mint}
	raw, _ := json.Marshal(body)
	ctx, cancel := contextTimeout()
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", errors.New("http " + resp.Status)
	}
	var out struct {
		Signature string `json:"signature"`
		Tx        string `json:"tx"`
		Sig       string `json:"sig"`
	}
	if json.Unmarshal(b, &out) != nil {
		return "", errors.New("executor: invalid JSON response")
	}
	sig := out.Signature
	if sig == "" {
		sig = out.Tx
	}
	if sig == "" {
		sig = out.Sig
	}
	if sig == "" {
		return "", errors.New("executor: no signature in response")
	}
	return sig, nil
}

func contextTimeout() (context.Context, context.CancelFunc) {
	ms := 1500
	if config.C != nil && config.C.TimeoutMS > 0 {
		ms = config.C.TimeoutMS
	}
	return context.WithTimeout(context.Background(), time.Duration(ms)*time.Millisecond)
}
