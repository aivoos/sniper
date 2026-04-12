package quote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"rlangga/internal/config"
)

var errNotConfigured = errors.New("quote: pump URL not configured")

// GetSellQuote returns simulate-sell SOL amount (PumpPortal → PumpAPI fallback).
func GetSellQuote(mint string) float64 {
	sol, err := pumpPortalQuote(mint)
	if err == nil {
		return sol
	}
	sol, _ = pumpApiQuote(mint)
	return sol
}

func pumpPortalQuote(mint string) (float64, error) {
	if config.C == nil || config.C.PumpPortalURL == "" || config.C.PumpPortalURL == "xxx" {
		return 0, errNotConfigured
	}
	return postQuote(config.C.PumpPortalURL, mint)
}

func pumpApiQuote(mint string) (float64, error) {
	if config.C == nil || config.C.PumpAPIURL == "" || config.C.PumpAPIURL == "xxx" {
		return 0, errNotConfigured
	}
	return postQuote(config.C.PumpAPIURL, mint)
}

func postQuote(baseURL, mint string) (float64, error) {
	u := baseURL
	if len(u) > 0 && u[len(u)-1] == '/' {
		u = u[:len(u)-1]
	}
	u = u + "/quote"
	body := map[string]string{"mint": mint}
	raw, _ := json.Marshal(body)
	ms := 1500
	if config.C != nil && config.C.TimeoutMS > 0 {
		ms = config.C.TimeoutMS
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(ms)*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(raw))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return 0, errors.New("quote: bad status")
	}
	var out struct {
		SOL    float64 `json:"sol"`
		Amount float64 `json:"amount"`
		Quote  float64 `json:"quote"`
	}
	if json.Unmarshal(b, &out) != nil {
		return 0, errors.New("quote: invalid JSON")
	}
	sol := out.SOL
	if sol == 0 {
		sol = out.Amount
	}
	if sol == 0 {
		sol = out.Quote
	}
	if sol == 0 {
		return 0, errors.New("quote: no sol field")
	}
	return sol, nil
}
