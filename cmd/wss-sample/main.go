// Command wss-sample: baca beberapa frame teks dari WebSocket (PumpAPI / PumpPortal) dan cetak JSON rapi.
// Pakai untuk melihat bentuk payload nyata sebelum set FILTER_WSS_*.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	url := flag.String("url", strings.TrimSpace(os.Getenv("PUMP_WS_URL")), "WebSocket URL (default: env PUMP_WS_URL, atau wss://stream.pumpapi.io)")
	n := flag.Int("n", 5, "jumlah frame teks yang dicetak")
	timeout := flag.Duration("timeout", 30*time.Second, "deadline baca per frame")
	subscribe := flag.String("subscribe", strings.TrimSpace(os.Getenv("PUMP_WS_SUBSCRIBE_JSON")), "JSON array subscribe (PumpPortal), contoh [{\"method\":\"subscribeNewToken\"}]")
	flag.Parse()

	u := strings.TrimSpace(*url)
	if u == "" {
		u = "wss://stream.pumpapi.io"
	}

	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, _, err := dialer.Dial(u, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	if err := sendSubscribe(conn, u, *subscribe); err != nil {
		fmt.Fprintf(os.Stderr, "subscribe: %v\n", err)
		os.Exit(1)
	}

	for i := 0; i < *n; i++ {
		_ = conn.SetReadDeadline(time.Now().Add(*timeout))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Fprintf(os.Stderr, "read: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("--- frame %d (%d bytes) ---\n", i+1, len(msg))
		var buf bytes.Buffer
		if json.Indent(&buf, msg, "", "  ") != nil {
			fmt.Println(string(msg))
		} else {
			fmt.Println(buf.String())
		}
	}
}

func sendSubscribe(conn *websocket.Conn, wsURL, subscribeJSON string) error {
	s := strings.TrimSpace(subscribeJSON)
	if s != "" {
		var arr []map[string]interface{}
		if err := json.Unmarshal([]byte(s), &arr); err != nil {
			return fmt.Errorf("parse -subscribe / PUMP_WS_SUBSCRIBE_JSON: %w", err)
		}
		for _, o := range arr {
			b, err := json.Marshal(o)
			if err != nil {
				return err
			}
			if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
				return err
			}
		}
		return nil
	}
	lower := strings.ToLower(wsURL)
	if strings.Contains(lower, "pumpportal") {
		for _, line := range []string{
			`{"method":"subscribeNewToken"}`,
			`{"method":"subscribeMigration"}`,
		} {
			if err := conn.WriteMessage(websocket.TextMessage, []byte(line)); err != nil {
				return err
			}
		}
	}
	return nil
}
