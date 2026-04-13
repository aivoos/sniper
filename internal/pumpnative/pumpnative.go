// Package pumpnative implements PumpPortal Lightning (/api/trade) and optional PumpAPI JSON flows.
// PumpAPI often returns raw tx bytes — if response is not JSON with a signature, an error is returned.
package pumpnative

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"rlangga/internal/config"
)

// ErrNonJSONResponse means the server returned a binary tx; sign & send locally or use a JSON gateway URL.
var ErrNonJSONResponse = errors.New("pumpnative: non-JSON response (binary transaction); use gateway or local signing")

// portalTradeBase returns trade URL without query; default PumpPortal Lightning.
func portalTradeBase(cfg *config.Config) string {
	b := strings.TrimSpace(cfg.PumpPortalURL)
	if b == "" {
		return "https://pumpportal.fun/api/trade"
	}
	return strings.TrimRight(b, "/")
}

// PortalTradeURL builds POST URL including api-key query when set.
func PortalTradeURL(cfg *config.Config) string {
	u := portalTradeBase(cfg)
	key := strings.TrimSpace(cfg.PumpPortalAPIKey)
	if key == "" {
		return u
	}
	sep := "?"
	if strings.Contains(u, "?") {
		sep = "&"
	}
	return u + sep + "api-key=" + url.QueryEscape(key)
}

// PortalBuy Lightning buy (POST trade URL).
func PortalBuy(cfg *config.Config, mint string) (string, error) {
	body := map[string]interface{}{
		"action":           "buy",
		"mint":             mint,
		"amount":           cfg.TradeSize,
		"denominatedInSol": "true",
		"slippage":         cfg.PumpSlippage,
		"priorityFee":      cfg.PumpPriorityFee,
		"pool":             "auto",
		"skipPreflight":    "false",
	}
	return postJSON(PortalTradeURL(cfg), body, cfg.TimeoutMS)
}

// PortalSell Lightning sell (100% token).
func PortalSell(cfg *config.Config, mint string) (string, error) {
	body := map[string]interface{}{
		"action":           "sell",
		"mint":             mint,
		"amount":           "100%",
		"denominatedInSol": "false",
		"slippage":         cfg.PumpSlippage,
		"priorityFee":      cfg.PumpPriorityFee,
		"pool":             "auto",
		"skipPreflight":    "false",
	}
	return postJSON(PortalTradeURL(cfg), body, cfg.TimeoutMS)
}

func apiBase(cfg *config.Config) string {
	b := strings.TrimSpace(cfg.PumpAPIURL)
	if b == "" {
		return "https://api.pumpapi.io"
	}
	return strings.TrimRight(b, "/")
}

// APIBuy POST https://api.pumpapi.io — identitas: PUMP_PRIVATE_KEY (privateKey, base58) disarankan, atau PUMP_WALLET_PUBLIC_KEY (legacy).
func APIBuy(cfg *config.Config, mint string) (string, error) {
	body := map[string]interface{}{
		"action":             "buy",
		"mint":               mint,
		"amount":             cfg.TradeSize,
		"denominatedInQuote": "true",
		"slippage":           cfg.PumpAPISlippage,
	}
	if err := applyPumpAPIIdentity(cfg, body); err != nil {
		return "", err
	}
	applyPumpAPIFeeAndOptions(cfg, body)
	return postJSON(apiBase(cfg), body, cfg.TimeoutMS)
}

// APISell sells 100% via PumpAPI JSON API when the response contains a signature.
func APISell(cfg *config.Config, mint string) (string, error) {
	body := map[string]interface{}{
		"action":             "sell",
		"mint":               mint,
		"amount":             "100%",
		"denominatedInQuote": "false",
		"slippage":           cfg.PumpAPISlippage,
	}
	if err := applyPumpAPIIdentity(cfg, body); err != nil {
		return "", err
	}
	applyPumpAPIFeeAndOptions(cfg, body)
	return postJSON(apiBase(cfg), body, cfg.TimeoutMS)
}

func applyPumpAPIIdentity(cfg *config.Config, body map[string]interface{}) error {
	if cfg == nil {
		return errors.New("pumpnative: nil config")
	}
	if sk := strings.TrimSpace(cfg.PumpPrivateKey); sk != "" {
		body["privateKey"] = sk
		return nil
	}
	if pk := strings.TrimSpace(cfg.WalletPublicKey); pk != "" {
		body["publicKey"] = pk
		return nil
	}
	return errors.New("pumpnative: set PUMP_PRIVATE_KEY (disarankan) or PUMP_WALLET_PUBLIC_KEY for PumpAPI")
}

func applyPumpAPIFeeAndOptions(cfg *config.Config, body map[string]interface{}) {
	if cfg == nil {
		return
	}
	if s := strings.TrimSpace(cfg.PumpAPIPriorityFeeMode); s != "" {
		body["priorityFee"] = s
	} else {
		body["priorityFee"] = cfg.PumpPriorityFee
	}
	if cfg.PumpAPIQuoteMint != "" {
		body["quoteMint"] = cfg.PumpAPIQuoteMint
	}
	if cfg.PumpAPIPoolID != "" {
		body["poolId"] = cfg.PumpAPIPoolID
	}
	if cfg.PumpAPIGuaranteedDelivery {
		body["guaranteedDelivery"] = true
	}
	if cfg.PumpAPIJitoTip > 0 {
		body["jitoTip"] = cfg.PumpAPIJitoTip
	}
	if cfg.PumpAPIMaxPriorityFee > 0 {
		body["maxPriorityFee"] = cfg.PumpAPIMaxPriorityFee
	}
}

// ShouldUsePortalNative is true when Lightning trade should be used (api-key + native mode).
func ShouldUsePortalNative(cfg *config.Config) bool {
	return cfg != nil && cfg.PumpNative && strings.TrimSpace(cfg.PumpPortalAPIKey) != ""
}

// ShouldUseAPINative is true for json pumpapi.io flow when private key or wallet public key is set.
func ShouldUseAPINative(cfg *config.Config) bool {
	if cfg == nil || !cfg.PumpNative {
		return false
	}
	if strings.TrimSpace(cfg.PumpPrivateKey) == "" && strings.TrimSpace(cfg.WalletPublicKey) == "" {
		return false
	}
	u := strings.TrimSpace(cfg.PumpAPIURL)
	if u == "" {
		return true
	}
	return strings.Contains(strings.ToLower(u), "pumpapi.io")
}

func postJSON(u string, body map[string]interface{}, timeoutMS int) (string, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	ms := timeoutMS
	if ms <= 0 {
		ms = 1500
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(ms)*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 300 {
		if sig, err := ParseTxSignatureJSON(b); err == nil && sig != "" {
			return sig, nil
		}
		return "", fmt.Errorf("pumpnative: http %s: %s", resp.Status, truncate(b, 200))
	}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if !strings.Contains(ct, "json") && len(b) > 0 && b[0] != '{' && b[0] != '[' {
		return "", ErrNonJSONResponse
	}
	return ParseTxSignatureJSON(b)
}

// ParseTxSignatureJSON extracts a transaction signature from flexible JSON shapes.
func ParseTxSignatureJSON(b []byte) (string, error) {
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return "", err
	}
	if errObj, ok := m["error"]; ok {
		return "", fmt.Errorf("pumpnative: %v", errObj)
	}
	for _, k := range []string{"signature", "tx", "sig", "txSig", "transactionSignature"} {
		if s, ok := m[k].(string); ok && s != "" {
			return s, nil
		}
	}
	if arr, ok := m["signatures"].([]interface{}); ok && len(arr) > 0 {
		if s, ok := arr[0].(string); ok && s != "" {
			return s, nil
		}
	}
	return "", errors.New("pumpnative: no signature in JSON response")
}

func truncate(b []byte, n int) string {
	s := string(b)
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
