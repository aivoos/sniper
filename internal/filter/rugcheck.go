// Package filter: gate BUY berdasarkan pemeriksaan on-chain (anti-rug / indikator honeypot).
// Bukan audit keamanan penuh — heuristik + RPC; selalu kombinasikan dengan exit PnL dan ukuran posisi.
package filter

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"rlangga/internal/config"
)

// Program SPL Token (legacy) dan Token-2022.
const (
	programTokenLegacy = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	programToken2022   = "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"
)

// AllowMint mengembalikan false jika mint tidak lolos filter (reason untuk log).
func AllowMint(ctx context.Context, mint string) (ok bool, reason string) {
	cfg := config.C
	if cfg == nil || !cfg.FilterAntiRug || cfg.RPCStub {
		return true, ""
	}
	mint = strings.TrimSpace(mint)
	if mint == "" {
		return false, "empty mint"
	}
	ep := rpcEndpoint(cfg)
	if ep == "" {
		if cfg.FilterRPCFailOpen {
			return true, ""
		}
		return false, "no RPC URL for filter"
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(cfg.TimeoutMS)*time.Millisecond)
	defer cancel()

	acc, err := getAccountInfo(ctx, ep, mint)
	if err != nil {
		if cfg.FilterRPCFailOpen {
			return true, ""
		}
		return false, "rpc getAccountInfo: " + err.Error()
	}
	if acc == nil {
		return false, "mint account missing"
	}
	owner := acc.Owner
	if owner != programTokenLegacy && owner != programToken2022 {
		return false, "mint owner not SPL Token program"
	}
	raw := acc.Data
	if len(raw) < 82 {
		return false, "invalid mint account data length"
	}

	freezeOpt := binary.LittleEndian.Uint32(raw[46:50])
	mintAuthOpt := binary.LittleEndian.Uint32(raw[0:4])
	supply := binary.LittleEndian.Uint64(raw[36:44])
	decimals := raw[44]
	init := raw[45] != 0

	if !init {
		return false, "mint not initialized"
	}
	if supply == 0 {
		return false, "zero supply"
	}
	if decimals > 18 {
		return false, "suspicious decimals"
	}

	if cfg.FilterRejectFreezeAuthority && freezeOpt != 0 {
		return false, "freeze authority set (honeypot/risk flag)"
	}
	if cfg.FilterRejectMintAuthority && mintAuthOpt != 0 {
		return false, "mint authority still set (inflation risk)"
	}

	if cfg.FilterMaxTopHolderPct > 0 {
		ok2, r := checkTopHolderPct(ctx, ep, mint, cfg.FilterMaxTopHolderPct, cfg.TimeoutMS)
		if !ok2 {
			return false, r
		}
	}

	return true, ""
}

type accountInfo struct {
	Owner string
	Data  []byte
}

func rpcEndpoint(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	if len(cfg.RPCURLs) > 0 {
		return cfg.RPCURLs[0]
	}
	return strings.TrimSpace(cfg.RPCURL)
}

func getAccountInfo(ctx context.Context, rpcURL, mint string) (*accountInfo, error) {
	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getAccountInfo",
		"params": []interface{}{
			mint,
			map[string]interface{}{"encoding": "base64"},
		},
	}
	var env struct {
		Result *struct {
			Value *struct {
				Owner string          `json:"owner"`
				Data  json.RawMessage `json:"data"`
			} `json:"value"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := rpcPost(ctx, rpcURL, body, &env); err != nil {
		return nil, err
	}
	if env.Error != nil {
		return nil, errors.New(env.Error.Message)
	}
	if env.Result == nil || env.Result.Value == nil {
		return nil, nil
	}
	v := env.Result.Value
	raw, err := decodeDataField(v.Data)
	if err != nil {
		return nil, err
	}
	return &accountInfo{Owner: v.Owner, Data: raw}, nil
}

func decodeDataField(raw json.RawMessage) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errors.New("empty data")
	}
	// Array [base64, "base64"]
	var arr []interface{}
	if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
		if s, ok := arr[0].(string); ok {
			return base64.StdEncoding.DecodeString(s)
		}
	}
	// String base64
	var s string
	if json.Unmarshal(raw, &s) == nil && s != "" {
		return base64.StdEncoding.DecodeString(s)
	}
	return nil, errors.New("cannot decode account data")
}

func rpcPost(ctx context.Context, rpcURL string, body interface{}, out interface{}) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, strings.NewReader(string(raw)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("rpc http " + resp.Status)
	}
	return json.Unmarshal(b, out)
}

func checkTopHolderPct(ctx context.Context, rpcURL, mint string, maxPct float64, timeoutMS int) (bool, string) {
	if maxPct <= 0 || maxPct > 100 {
		return true, ""
	}
	ms := timeoutMS
	if ms <= 0 {
		ms = 1500
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(ms)*time.Millisecond)
	defer cancel()

	supplyStr, err := getTokenSupplyAmount(ctx, rpcURL, mint)
	if err != nil {
		if config.C != nil && config.C.FilterRPCFailOpen {
			return true, ""
		}
		return false, "token supply: " + err.Error()
	}
	total, ok := new(big.Int).SetString(supplyStr, 10)
	if !ok || total.Sign() == 0 {
		return false, "invalid total supply"
	}

	largestStr, err := getLargestHolderAmount(ctx, rpcURL, mint)
	if err != nil {
		if config.C != nil && config.C.FilterRPCFailOpen {
			return true, ""
		}
		return false, "largest accounts: " + err.Error()
	}
	largest, ok := new(big.Int).SetString(largestStr, 10)
	if !ok {
		return false, "invalid largest holder amount"
	}

	// pct = largest * 100 / total
	num := new(big.Int).Mul(largest, big.NewInt(100))
	pct := new(big.Float).Quo(new(big.Float).SetInt(num), new(big.Float).SetInt(total))
	pf, _ := pct.Float64()
	if pf > maxPct {
		return false, "top holder concentration too high"
	}
	return true, ""
}

func getTokenSupplyAmount(ctx context.Context, rpcURL, mint string) (string, error) {
	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getTokenSupply",
		"params":  []interface{}{mint},
	}
	var env struct {
		Result *struct {
			Value struct {
				Amount string `json:"amount"`
			} `json:"value"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := rpcPost(ctx, rpcURL, body, &env); err != nil {
		return "", err
	}
	if env.Error != nil {
		return "", errors.New(env.Error.Message)
	}
	if env.Result == nil {
		return "", errors.New("no supply")
	}
	return env.Result.Value.Amount, nil
}

func getLargestHolderAmount(ctx context.Context, rpcURL, mint string) (string, error) {
	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getTokenLargestAccounts",
		"params": []interface{}{
			mint,
			map[string]interface{}{"commitment": "confirmed"},
		},
	}
	var env struct {
		Result *struct {
			Value []struct {
				Amount string `json:"amount"`
			} `json:"value"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := rpcPost(ctx, rpcURL, body, &env); err != nil {
		return "", err
	}
	if env.Error != nil {
		return "", errors.New(env.Error.Message)
	}
	if env.Result == nil || len(env.Result.Value) == 0 {
		return "", errors.New("no holders")
	}
	return env.Result.Value[0].Amount, nil
}
