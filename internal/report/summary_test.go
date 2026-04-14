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

func TestLogTradeRealtime(t *testing.T) {
	LogTradeRealtime(store.Trade{Mint: "m", BotName: "b", PnLSOL: 0.01, Percent: 1, DurationSec: 2}, true, nil)
	LogTradeRealtime(store.Trade{Mint: "m2"}, false, nil)
	LogTradeRealtime(store.Trade{Mint: "m3"}, false, io.EOF)
}

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

func TestNotifyBotStarted_MockTelegram(t *testing.T) {
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
	t.Setenv("TELEGRAM_BOT_TOKEN", "BOT")
	t.Setenv("TELEGRAM_CHAT_ID", "99")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if err := NotifyBotStarted(); err != nil {
		t.Fatal(err)
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

func TestSendSummaryWithTrades_ExitBreakdown(t *testing.T) {
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
	t.Setenv("TELEGRAM_BOT_TOKEN", "T")
	t.Setenv("TELEGRAM_CHAT_ID", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	trades := []store.Trade{
		{Mint: "So11111111111111111111111111111111111111112", PnLSOL: 0.2, Percent: 2, ExitReason: "take_profit", TS: now},
		{Mint: "So11111111111111111111111111111111111111112", PnLSOL: -0.1, Percent: -1, ExitReason: "stop_loss", TS: now - 1},
	}
	if err := SendSummaryWithTrades(aggregate.Stats{Total: 2, Winrate: 50, TotalPnL: 0.1}, 1, trades); err != nil {
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
	t.Setenv("REPORT_INTERVAL_MIN", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_ = redisx.Client.FlushDB(ctx).Err()
	past := strconv.FormatInt(time.Now().Unix()-4000, 10)
	_ = redisx.Client.Set(ctx, keyReportLastSent, past, 0).Err()
	tr := store.Trade{Mint: "m", BuySOL: 0.1, SellSOL: 0.1, PnLSOL: 0, Percent: 0, DurationSec: 1, TS: 7}
	if _, err := store.SaveTrade(tr); err != nil {
		t.Fatal(err)
	}
	if err := NotifyTradeSaved(); err == nil {
		t.Fatal("expected send error")
	}
	v, _ := redisx.Client.Get(ctx, keyReportLastSent).Result()
	lastSent, _ := strconv.ParseInt(v, 10, 64)
	if lastSent != time.Now().Unix() && lastSent > time.Now().Unix()-2 {
		t.Fatal("timestamp should not update on failure")
	}
}

func TestNotifyTradeSaved_IntervalTrigger(t *testing.T) {
	testutil.UseMiniredis(t)
	t.Setenv("RPC_STUB", "1")
	t.Setenv("REPORT_INTERVAL_MIN", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_ = redisx.Client.FlushDB(ctx).Err()
	past := strconv.FormatInt(time.Now().Unix()-4000, 10)
	_ = redisx.Client.Set(ctx, keyReportLastSent, past, 0).Err()
	tr := store.Trade{Mint: "m", BuySOL: 0.1, SellSOL: 0.1, PnLSOL: 0, Percent: 0, DurationSec: 1, TS: 99}
	if _, err := store.SaveTrade(tr); err != nil {
		t.Fatal(err)
	}
	if err := NotifyTradeSaved(); err != nil {
		t.Fatal(err)
	}
}

func TestResetReportState(t *testing.T) {
	testutil.UseMiniredis(t)
	ctx := context.Background()
	_ = redisx.Client.Set(ctx, keyReportCount, "5", 0).Err()
	_ = redisx.Client.Set(ctx, keyReportLastSent, "999", 0).Err()
	if err := ResetReportState(ctx); err != nil {
		t.Fatal(err)
	}
	n, err := redisx.Client.Exists(ctx, keyReportCount, keyReportLastSent).Result()
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("expected keys removed, exists=%d", n)
	}
}

func TestResetReportState_NoRedis(t *testing.T) {
	prev := redisx.Client
	redisx.Client = nil
	t.Cleanup(func() { redisx.Client = prev })
	if err := ResetReportState(context.Background()); err == nil {
		t.Fatal("expected error without redis")
	}
}

func TestNotifyTradeSaved_NoTriggerBeforeInterval(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	restore := SetTelegramAPIBase(srv.URL)
	t.Cleanup(restore)

	testutil.UseMiniredis(t)
	t.Setenv("RPC_STUB", "1")
	t.Setenv("TELEGRAM_BOT_TOKEN", "T")
	t.Setenv("TELEGRAM_CHAT_ID", "1")
	t.Setenv("REPORT_INTERVAL_MIN", "10")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_ = redisx.Client.FlushDB(ctx).Err()
	now := strconv.FormatInt(time.Now().Unix(), 10)
	_ = redisx.Client.Set(ctx, keyReportLastSent, now, 0).Err()
	tr := store.Trade{Mint: "m", BuySOL: 0.1, SellSOL: 0.1, PnLSOL: 0, Percent: 0, DurationSec: 1, TS: 42}
	if _, err := store.SaveTrade(tr); err != nil {
		t.Fatal(err)
	}
	if err := NotifyTradeSaved(); err != nil {
		t.Fatal(err)
	}
	if hits != 0 {
		t.Fatalf("should not send before interval, hits=%d", hits)
	}
}
