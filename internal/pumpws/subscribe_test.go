package pumpws

import "testing"

func TestSubscribePayloads_PumpPortalDefault(t *testing.T) {
	opt := Options{URL: "wss://pumpportal.fun/api/data"}
	out := subscribePayloads(opt)
	if len(out) != 2 {
		t.Fatalf("len=%d", len(out))
	}
}

func TestSubscribePayloads_PumpApiNoDefaultSubscribe(t *testing.T) {
	opt := Options{URL: "wss://stream.pumpapi.io/"}
	out := subscribePayloads(opt)
	if len(out) != 0 {
		t.Fatalf("expected no default subscribe for pumpapi, got %d", len(out))
	}
}

func TestSubscribePayloads_CustomJSON(t *testing.T) {
	opt := Options{
		URL:           "wss://pumpportal.fun/api/data",
		SubscribeJSON: `[{"method":"subscribeNewToken"}]`,
	}
	out := subscribePayloads(opt)
	if len(out) != 1 || string(out[0]) != `{"method":"subscribeNewToken"}` {
		t.Fatalf("%s", string(out[0]))
	}
}
