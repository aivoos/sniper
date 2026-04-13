package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"rlangga/internal/config"
)

// callRPC mengirim satu permintaan JSON-RPC ke endpoint pertama yang mengembalikan hasil valid.
func callRPC(method string, params []interface{}, resultDest interface{}) error {
	if config.C == nil {
		return errors.New("rpc: config not loaded")
	}
	endpoints := rpcEndpoints()
	if len(endpoints) == 0 {
		return errors.New("rpc: no endpoints")
	}
	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	timeout := time.Duration(config.C.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}
	var lastErr error
	for _, url := range endpoints {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
		if err != nil {
			cancel()
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		cancel()
		if err != nil {
			lastErr = err
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("rpc: HTTP %d", resp.StatusCode)
			continue
		}
		var envelope struct {
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
			Result json.RawMessage `json:"result"`
		}
		if json.Unmarshal(b, &envelope) != nil {
			lastErr = errors.New("rpc: invalid JSON envelope")
			continue
		}
		if envelope.Error != nil {
			lastErr = fmt.Errorf("rpc: %s", envelope.Error.Message)
			continue
		}
		if json.Unmarshal(envelope.Result, resultDest) != nil {
			lastErr = errors.New("rpc: parse result")
			continue
		}
		return nil
	}
	if lastErr == nil {
		lastErr = errors.New("rpc: all endpoints failed")
	}
	return lastErr
}
