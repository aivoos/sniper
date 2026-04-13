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

// GetSellQuote returns simulate-sell SOL amount (smart race antara PumpPortal dan PumpAPI).
func GetSellQuote(mint string) float64 {
	sol, _ := GetSellQuoteWithTime(mint)
	return sol
}

// GetSellQuoteWithTime returns the quote dari provider tercepat (lihat RaceQuote) dan waktu respons.
func GetSellQuoteWithTime(mint string) (sol float64, receivedAt time.Time) {
	sol, receivedAt, _, ok := RaceQuote(mint)
	if !ok {
		return 0, time.Time{}
	}
	return sol, receivedAt
}

// RaceQuote memanggil quote PumpPortal dan PumpAPI secara paralel. Di antara respons sukses,
// yang receivedAt lebih awal menang (latensi lebih rendah). preferPortal=true jika Portal menang.
// Dipakai executor untuk urutan BUY/SELL tanpa menembak dua transaksi on-chain sekaligus.
func RaceQuote(mint string) (sol float64, receivedAt time.Time, preferPortal bool, ok bool) {
	type res struct {
		src string
		sol float64
		at  time.Time
		err error
	}
	ch := make(chan res, 2)
	go func() {
		s, a, e := pumpPortalQuote(mint)
		ch <- res{src: "portal", sol: s, at: a, err: e}
	}()
	go func() {
		s, a, e := pumpApiQuote(mint)
		ch <- res{src: "api", sol: s, at: a, err: e}
	}()
	var portalR, apiR res
	for i := 0; i < 2; i++ {
		r := <-ch
		if r.src == "portal" {
			portalR = r
		} else {
			apiR = r
		}
	}
	pOk := portalR.err == nil && portalR.sol > 0
	aOk := apiR.err == nil && apiR.sol > 0
	if !pOk && !aOk {
		return 0, time.Time{}, false, false
	}
	if pOk && !aOk {
		return portalR.sol, portalR.at, true, true
	}
	if !pOk && aOk {
		return apiR.sol, apiR.at, false, true
	}
	// Keduanya sukses: lebih cepat (earlier receivedAt) menang; seri → Portal.
	if portalR.at.Before(apiR.at) || portalR.at.Equal(apiR.at) {
		return portalR.sol, portalR.at, true, true
	}
	return apiR.sol, apiR.at, false, true
}

func pumpPortalQuote(mint string) (float64, time.Time, error) {
	if config.C == nil {
		return 0, time.Time{}, errNotConfigured
	}
	base := config.C.PumpPortalURL
	if config.C.PumpPortalQuoteURL != "" {
		base = config.C.PumpPortalQuoteURL
	}
	if config.IsUnsetPumpURL(base) {
		return 0, time.Time{}, errNotConfigured
	}
	return postQuote(base, mint)
}

func pumpApiQuote(mint string) (float64, time.Time, error) {
	if config.C == nil {
		return 0, time.Time{}, errNotConfigured
	}
	base := config.C.PumpAPIURL
	if config.C.PumpAPIQuoteURL != "" {
		base = config.C.PumpAPIQuoteURL
	}
	if config.IsUnsetPumpURL(base) {
		return 0, time.Time{}, errNotConfigured
	}
	return postQuote(base, mint)
}

func postQuote(baseURL, mint string) (float64, time.Time, error) {
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
		return 0, time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, time.Time{}, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	receivedAt := time.Now()
	if resp.StatusCode >= 300 {
		return 0, time.Time{}, errors.New("quote: bad status")
	}
	var out struct {
		SOL    float64 `json:"sol"`
		Amount float64 `json:"amount"`
		Quote  float64 `json:"quote"`
	}
	if json.Unmarshal(b, &out) != nil {
		return 0, time.Time{}, errors.New("quote: invalid JSON")
	}
	sol := out.SOL
	if sol == 0 {
		sol = out.Amount
	}
	if sol == 0 {
		sol = out.Quote
	}
	if sol == 0 {
		return 0, time.Time{}, errors.New("quote: no sol field")
	}
	return sol, receivedAt, nil
}
