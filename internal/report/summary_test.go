package report

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"rlangga/internal/aggregate"
	"rlangga/internal/config"
	"rlangga/internal/redisx"
	"rlangga/internal/store"
	"rlangga/internal/testutil"
)

func TestNotifyTradeSaved_NilConfig(t *testing.T) {
	testutil.UseMiniredis(t)
	prev := config.C
	config.C = nil
	t.Cleanup(func() { config.C = prev })
	if err := NotifyTradeSaved(); err != nil {
		t.Fatal(err)
	}
}

func TestNotifyTradeSaved_NoRedis(t *testing.T) {
	prevC := config.C
	prevR := redisx.Client
	config.C = &config.Config{}
	redisx.Client = nil
	t.Cleanup(func() {
		config.C = prevC
		redisx.Client = prevR
	})
	if err := NotifyTradeSaved(); err != nil {
		t.Fatal(err)
	}
}

func TestSendSummary_NoTelegram(t *testing.T) {
	if err := SendSummary(aggregate.Stats{Total: 1, Winrate: 50}, 0); err != nil {
		t.Fatal(err)
	}
}

func TestSendPlainMessage_NoTelegram(t *testing.T) {
	if err := SendPlainMessage("x"); err != nil {
		t.Fatal(err)
	}
}

func TestSendPlainMessage_NilConfig(t *testing.T) {
	prev := config.C
	config.C = nil
	t.Cleanup(func() { config.C = prev })
	if err := SendPlainMessage("x"); err != nil {
		t.Fatal(err)
	}
}

func TestSendPlainMessage_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)
	restore := SetTelegramAPIBase(srv.URL)
	t.Cleanup(restore)
	t.Setenv("RPC_STUB", "1")
	t.Setenv("TELEGRAM_BOT_TOKEN", "T")
	t.Setenv("TELEGRAM_CHAT_ID", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if err := SendPlainMessage("e"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSendPlainMessage_MockTelegram(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	restore := SetTelegramAPIBase(srv.URL)
	t.Cleanup(restore)

	t.Setenv("RPC_STUB", "1")
	t.Setenv("TELEGRAM_BOT_TOKEN", "T")
	t.Setenv("TELEGRAM_CHAT_ID", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if err := SendPlainMessage("alert"); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("hits=%d", hits)
	}
}

func TestSendSummary_MockTelegram(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	restore := SetTelegramAPIBase(srv.URL)
	t.Cleanup(restore)

	t.Setenv("RPC_STUB", "1")
	t.Setenv("TELEGRAM_BOT_TOKEN", "TEST")
	t.Setenv("TELEGRAM_CHAT_ID", "123")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if err := SendSummary(aggregate.Stats{Total: 2, Winrate: 50, TotalPnL: 0.01}, 1); err != nil {
		t.Fatal(err)
	}
}

func TestSendSummary_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	restore := SetTelegramAPIBase(srv.URL)
	t.Cleanup(restore)

	t.Setenv("RPC_STUB", "1")
	t.Setenv("TELEGRAM_BOT_TOKEN", "T")
	t.Setenv("TELEGRAM_CHAT_ID", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if err := SendSummary(aggregate.Stats{}, 0); err == nil {
		t.Fatal("expected HTTP error")
	}
}

func TestNotifyTradeSaved_SendFailsNoReset(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(bad.Close)
	restore := SetTelegramAPIBase(bad.URL)
	t.Cleanup(restore)

	testutil.UseMiniredis(t)
	t.Setenv("RPC_STUB", "1")
	t.Setenv("TELEGRAM_BOT_TOKEN", "T")
	t.Setenv("TELEGRAM_CHAT_ID", "1")
	t.Setenv("REPORT_EVERY_N_TRADES", "1")
	t.Setenv("REPORT_INTERVAL_MIN", "0")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	_ = redisx.Client.FlushDB(context.Background()).Err()
	tr := store.Trade{Mint: "m", BuySOL: 0.1, SellSOL: 0.1, PnLSOL: 0, Percent: 0, DurationSec: 1, TS: 7}
	if _, err := store.SaveTrade(tr); err != nil {
		t.Fatal(err)
	}
	if err := NotifyTradeSaved(); err == nil {
		t.Fatal("expected send error")
	}
	n, _ := redisx.Client.Get(context.Background(), keyReportCount).Int64()
	if n != 1 {
		t.Fatalf("count should not reset on failure: %d", n)
	}
}

func TestNotifyTradeSaved_IntervalTrigger(t *testing.T) {
	testutil.UseMiniredis(t)
	t.Setenv("RPC_STUB", "1")
	t.Setenv("REPORT_EVERY_N_TRADES", "9999")
	t.Setenv("REPORT_INTERVAL_MIN", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_ = redisx.Client.FlushDB(ctx).Err()
	past := strconv.FormatInt(time.Now().Unix()-4000, 10)
	_ = redisx.Client.Set(ctx, keyReportLastSent, past, 0).Err()
	_ = redisx.Client.Set(ctx, keyReportCount, "0", 0).Err()
	tr := store.Trade{Mint: "m", BuySOL: 0.1, SellSOL: 0.1, PnLSOL: 0, Percent: 0, DurationSec: 1, TS: 99}
	if _, err := store.SaveTrade(tr); err != nil {
		t.Fatal(err)
	}
	if err := NotifyTradeSaved(); err != nil {
		t.Fatal(err)
	}
}

func TestNotifyTradeSaved_EveryNTriggers(t *testing.T) {
	testutil.UseMiniredis(t)
	t.Setenv("RPC_STUB", "1")
	t.Setenv("REPORT_EVERY_N_TRADES", "1")
	t.Setenv("REPORT_INTERVAL_MIN", "0")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	_ = redisx.Client.FlushDB(context.Background()).Err()
	tr := store.Trade{Mint: "m", BuySOL: 0.1, SellSOL: 0.1, PnLSOL: 0, Percent: 0, DurationSec: 1, TS: 42}
	if _, err := store.SaveTrade(tr); err != nil {
		t.Fatal(err)
	}
	if err := NotifyTradeSaved(); err != nil {
		t.Fatal(err)
	}
}
