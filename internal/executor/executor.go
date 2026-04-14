package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"rlangga/internal/config"
	"rlangga/internal/log"
	"rlangga/internal/pumpnative"
	"rlangga/internal/quote"
	"rlangga/internal/rpc"
	"rlangga/internal/wallet"
)

var errNotConfigured = errors.New("executor: pump URL not configured")

// BuyAndValidate memakai quote.RaceQuote (PumpPortal vs PumpAPI) untuk urutan provider;
// hanya satu jalur BUY on-chain per mint: jika provider pertama mengembalikan signature, provider kedua
// tidak dipanggil walau konfirmasi RPC lambat (hindari double BUY).
func BuyAndValidate(mint string) bool {
	sim := config.C != nil && config.C.SimulateEngine
	_, _, preferPortal, ok := quote.RaceQuote(mint)
	if sim {
		log.Info("[SIM-ENGINE] BUY mint=" + mint + " (no on-chain tx, quote race done)")
		return true
	}
	if !ok {
		if ok, try := buyOutcome(pumpPortalBuy, mint); ok {
			return true
		} else if !try {
			return false
		}
		if ok, try := buyOutcome(pumpApiBuy, mint); ok {
			return true
		} else if !try {
			return false
		}
		return false
	}
	if preferPortal {
		if ok, try := buyOutcome(pumpPortalBuy, mint); ok {
			return true
		} else if !try {
			return false
		}
		if ok, try := buyOutcome(pumpApiBuy, mint); ok {
			return true
		} else if !try {
			return false
		}
		return false
	}
	if ok, try := buyOutcome(pumpApiBuy, mint); ok {
		return true
	} else if !try {
		return false
	}
	if ok, try := buyOutcome(pumpPortalBuy, mint); ok {
		return true
	} else if !try {
		return false
	}
	return false
}

// buyOutcome: confirmed=true sukses; tryAlternate=true boleh coba provider lain; false=jangan (tx sudah dikirim).
func buyOutcome(buy func(string) (string, error), mint string) (confirmed bool, tryAlternate bool) {
	sig, err := buy(mint)
	if err != nil || sig == "" {
		return false, true
	}
	if rpc.ConfirmTransaction(sig) {
		return true, false
	}
	log.Info("executor: BUY signature submitted; skip alternate provider (avoid double spend): " + sig)
	return false, false
}

// SafeSellWithValidation: race quote (sama seperti BUY), urutan SELL per provider, retry beberapa putaran.
// Jika satu provider mengembalikan signature, putaran itu tidak memanggil provider kedua (hindari double SELL).
func SafeSellWithValidation(mint string) bool {
	sim := config.C != nil && config.C.SimulateEngine
	_, _, preferPortal, ok := quote.RaceQuote(mint)
	if sim {
		log.Info("[SIM-ENGINE] SELL mint=" + mint + " (no on-chain tx, quote race done)")
		return true
	}
	var first, second func(string) (string, error)
	if ok && preferPortal {
		first, second = sellPortal, sellAPI
	} else if ok && !preferPortal {
		first, second = sellAPI, sellPortal
	} else {
		first, second = sellPortal, sellAPI
	}
	const maxRounds = 8
	for i := 0; i < maxRounds; i++ {
		if ok, try := sellOutcome(first, mint); ok {
			return true
		} else if !try {
			return false
		}
		if ok, try := sellOutcome(second, mint); ok {
			return true
		} else if !try {
			return false
		}
		time.Sleep(500 * time.Millisecond)
	}
	log.Error("SELL FAILED: " + mint)
	return false
}

func sellOutcome(sell func(string) (string, error), mint string) (confirmed bool, tryAlternate bool) {
	sig, err := sell(mint)
	if err != nil || sig == "" {
		return false, true
	}
	if rpc.ConfirmTransaction(sig) {
		return true, false
	}
	log.Info("executor: SELL signature submitted; skip alternate provider this round: " + sig)
	return false, false
}

func buyAmount() float64 {
	return wallet.GetTradeSize()
}

func pumpPortalBuy(mint string) (string, error) {
	if config.C == nil {
		return "", errNotConfigured
	}
	cfg := config.C
	if pumpnative.ShouldUsePortalNative(cfg) {
		sig, err := pumpnative.PortalBuy(cfg, mint, buyAmount())
		if err == nil {
			return sig, nil
		}
		if !config.IsUnsetPumpURL(cfg.PumpPortalURL) && !strings.Contains(cfg.PumpPortalURL, "/api/trade") {
			if s, e := postMint(cfg.PumpPortalURL, mint); e == nil {
				return s, nil
			}
		}
		return "", err
	}
	if config.IsUnsetPumpURL(cfg.PumpPortalURL) {
		return "", errNotConfigured
	}
	return postMint(cfg.PumpPortalURL, mint)
}

func pumpApiBuy(mint string) (string, error) {
	if config.C == nil {
		return "", errNotConfigured
	}
	cfg := config.C
	if pumpnative.ShouldUseAPINative(cfg) {
		sig, err := pumpnative.APIBuy(cfg, mint, buyAmount())
		if err == nil {
			return sig, nil
		}
		if !errors.Is(err, pumpnative.ErrNonJSONResponse) {
			return "", err
		}
	}
	if config.IsUnsetPumpURL(cfg.PumpAPIURL) {
		return "", errNotConfigured
	}
	return postMint(cfg.PumpAPIURL, mint)
}

func sellPortal(mint string) (string, error) {
	if config.C == nil {
		return "", errNotConfigured
	}
	cfg := config.C
	if pumpnative.ShouldUsePortalNative(cfg) {
		sig, err := pumpnative.PortalSell(cfg, mint)
		if err == nil {
			return sig, nil
		}
		if !config.IsUnsetPumpURL(cfg.PumpPortalURL) && !strings.Contains(cfg.PumpPortalURL, "/api/trade") {
			u := cfg.PumpPortalURL
			if u[len(u)-1] == '/' {
				u = u[:len(u)-1]
			}
			if s, e := postRaw(u+"/sell", mint); e == nil {
				return s, nil
			}
		}
		return "", err
	}
	if config.IsUnsetPumpURL(cfg.PumpPortalURL) {
		return "", errNotConfigured
	}
	u := cfg.PumpPortalURL
	if u[len(u)-1] == '/' {
		u = u[:len(u)-1]
	}
	return postRaw(u+"/sell", mint)
}

func sellAPI(mint string) (string, error) {
	if config.C == nil {
		return "", errNotConfigured
	}
	cfg := config.C
	if pumpnative.ShouldUseAPINative(cfg) {
		sig, err := pumpnative.APISell(cfg, mint)
		if err == nil {
			return sig, nil
		}
		if !errors.Is(err, pumpnative.ErrNonJSONResponse) {
			return "", err
		}
	}
	if config.IsUnsetPumpURL(cfg.PumpAPIURL) {
		return "", errNotConfigured
	}
	u := cfg.PumpAPIURL
	if u[len(u)-1] == '/' {
		u = u[:len(u)-1]
	}
	return postRaw(u+"/sell", mint)
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
