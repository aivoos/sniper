package pumpws

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"rlangga/internal/log"
)

var dialer = &websocket.Dialer{
	Proxy:            http.ProxyFromEnvironment,
	HandshakeTimeout: 15 * time.Second,
}

// Options snapshot untuk satu koneksi WS (hindari baca config.C dari goroutine).
type Options struct {
	URL           string
	SubscribeJSON string
}

// Run menjaga koneksi WebSocket: reconnect dengan backoff, subscribe PumpPortal jika perlu.
// onMessage dipanggil untuk setiap frame teks/binary. Jika URL kosong, return segera.
func Run(ctx context.Context, opt Options, onMessage func([]byte)) {
	if strings.TrimSpace(opt.URL) == "" {
		return
	}
	go runLoop(ctx, opt, onMessage)
}

func runLoop(ctx context.Context, opt Options, onMessage func([]byte)) {
	url := opt.URL
	backoff := time.Second
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, _, err := dialer.Dial(url, nil)
		if err != nil {
			log.Error("pumpws: dial: " + err.Error())
			sleepBackoff(ctx, &backoff)
			continue
		}
		backoff = time.Second
		log.Info("pumpws: connected " + url)

		for _, payload := range subscribePayloads(opt) {
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				log.Error("pumpws: subscribe write: " + err.Error())
			}
		}

		readErr := readUntilClose(ctx, conn, onMessage)
		_ = conn.Close()
		if readErr != nil && ctx.Err() == nil {
			log.Error("pumpws: read: " + readErr.Error())
		}
		if ctx.Err() != nil {
			return
		}
		sleepBackoff(ctx, &backoff)
	}
}

func sleepBackoff(ctx context.Context, backoff *time.Duration) {
	t := *backoff
	timer := time.NewTimer(t)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
	if t < 30*time.Second {
		t *= 2
	}
	*backoff = t
}

func readUntilClose(ctx context.Context, conn *websocket.Conn, onMessage func([]byte)) error {
	const readTimeout = 60 * time.Second
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		if onMessage == nil {
			continue
		}
		if mt != websocket.TextMessage && mt != websocket.BinaryMessage {
			continue
		}
		onMessage(msg)
	}
}

func subscribePayloads(opt Options) [][]byte {
	if opt.SubscribeJSON != "" {
		var arr []map[string]interface{}
		if err := json.Unmarshal([]byte(opt.SubscribeJSON), &arr); err != nil {
			log.Error("pumpws: PUMP_WS_SUBSCRIBE_JSON invalid JSON, using defaults for pumpportal if applicable")
			return defaultPumpPortalSubs(opt.URL)
		}
		out := make([][]byte, 0, len(arr))
		for _, o := range arr {
			b, err := json.Marshal(o)
			if err != nil {
				continue
			}
			out = append(out, b)
		}
		if len(out) > 0 {
			return out
		}
	}
	return defaultPumpPortalSubs(opt.URL)
}

func defaultPumpPortalSubs(wsURL string) [][]byte {
	lower := strings.ToLower(wsURL)
	if strings.Contains(lower, "pumpportal") {
		return [][]byte{
			[]byte(`{"method":"subscribeNewToken"}`),
			[]byte(`{"method":"subscribeMigration"}`),
		}
	}
	// PumpAPI stream (stream.pumpapi.io): tanpa subscribe default — set PUMP_WS_SUBSCRIBE_JSON / FALLBACK jika perlu.
	if strings.Contains(lower, "pumpapi") {
		return nil
	}
	return nil
}
