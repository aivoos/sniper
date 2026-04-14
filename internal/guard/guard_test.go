package guard

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"rlangga/internal/config"
	"rlangga/internal/redisx"
	"rlangga/internal/report"
	"rlangga/internal/testutil"
)

func TestUpdateDailyLoss_OnlyNegative(t *testing.T) {
	testutil.UseMiniredis(t)
	t.Setenv("RPC_STUB", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if err := UpdateDailyLoss(0.05); err != nil {
		t.Fatal(err)
	}
	if err := UpdateDailyLoss(-0.1); err != nil {
		t.Fatal(err)
	}
	loss, err := DailyLossSOL()
	if err != nil {
		t.Fatal(err)
	}
	if loss != 0.1 {
		t.Fatalf("loss=%v", loss)
	}
}

func TestCanTrade_KillSwitchSendsTelegramOnce(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	restoreTG := report.SetTelegramAPIBase(srv.URL)
	t.Cleanup(restoreTG)

	testutil.UseMiniredis(t)
	t.Setenv("RPC_STUB", "1")
	t.Setenv("MAX_DAILY_LOSS", "0.5")
	t.Setenv("TELEGRAM_BOT_TOKEN", "T")
	t.Setenv("TELEGRAM_CHAT_ID", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	_ = redisx.Client.Set(context.Background(), keyDailyLoss(), "0.6", 0).Err()
	if CanTrade(100) {
		t.Fatal("expected block")
	}
	if CanTrade(100) {
		t.Fatal("expected block")
	}
	if hits != 1 {
		t.Fatalf("telegram hits want 1 got %d", hits)
	}
}

func TestIsKillSwitchTriggered_DisabledWhenMaxZero(t *testing.T) {
	testutil.UseMiniredis(t)
	t.Setenv("RPC_STUB", "1")
	t.Setenv("MAX_DAILY_LOSS", "0")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	_ = redisx.Client.Set(context.Background(), keyDailyLoss(), "999", 0).Err()
	if IsKillSwitchTriggered() {
		t.Fatal("max 0 disables kill switch")
	}
}

func TestIsKillSwitchTriggered(t *testing.T) {
	testutil.UseMiniredis(t)
	t.Setenv("RPC_STUB", "1")
	t.Setenv("MAX_DAILY_LOSS", "0.5")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	_ = redisx.Client.Set(context.Background(), keyDailyLoss(), "0.6", 0).Err()
	if !IsKillSwitchTriggered() {
		t.Fatal("expected kill switch")
	}
}

func TestIsKillSwitchTriggered_SixDecimalRounding(t *testing.T) {
	testutil.UseMiniredis(t)
	t.Setenv("RPC_STUB", "1")
	t.Setenv("MAX_DAILY_LOSS", "0.5")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	_ = redisx.Client.Set(context.Background(), keyDailyLoss(), "0.4999994", 0).Err()
	if IsKillSwitchTriggered() {
		t.Fatal("below max after rounding")
	}
	_ = redisx.Client.Set(context.Background(), keyDailyLoss(), "0.4999995", 0).Err()
	if !IsKillSwitchTriggered() {
		t.Fatal("at max after rounding")
	}
}

func TestCanTrade_NilConfig(t *testing.T) {
	prev := config.C
	config.C = nil
	t.Cleanup(func() { config.C = prev })
	if CanTrade(100) {
		t.Fatal("nil config")
	}
}

func TestCanTrade_EnableTrading(t *testing.T) {
	testutil.UseMiniredis(t)
	t.Setenv("RPC_STUB", "1")
	t.Setenv("ENABLE_TRADING", "false")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if CanTrade(100) {
		t.Fatal("trading disabled")
	}
}

func TestHasEnoughBalance_NilConfig(t *testing.T) {
	prev := config.C
	config.C = nil
	t.Cleanup(func() { config.C = prev })
	if HasEnoughBalance(100) {
		t.Fatal("nil config")
	}
}

func TestDailyTradeCount_ParseError(t *testing.T) {
	testutil.UseMiniredis(t)
	t.Setenv("RPC_STUB", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	_ = redisx.Client.Set(context.Background(), keyDailyTrades(), "nope", 0).Err()
	if _, err := DailyTradeCount(); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestDailyLossSOL_InvalidFloat(t *testing.T) {
	testutil.UseMiniredis(t)
	t.Setenv("RPC_STUB", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	_ = redisx.Client.Set(context.Background(), keyDailyLoss(), "x", 0).Err()
	if _, err := DailyLossSOL(); err == nil {
		t.Fatal("expected error")
	}
}

func TestCanTrade_MinBalance(t *testing.T) {
	testutil.UseMiniredis(t)
	t.Setenv("RPC_STUB", "1")
	t.Setenv("MIN_BALANCE", "2")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	if CanTrade(1) {
		t.Fatal("low balance")
	}
	if !CanTrade(3) {
		t.Fatal("enough balance")
	}
}

func TestCanTrade_MaxDailyTrades(t *testing.T) {
	testutil.UseMiniredis(t)
	t.Setenv("RPC_STUB", "1")
	t.Setenv("MAX_DAILY_TRADES", "2")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	_ = redisx.Client.Set(context.Background(), keyDailyTrades(), "2", 0).Err()
	if CanTrade(10) {
		t.Fatal("quota exhausted")
	}
	_ = redisx.Client.Set(context.Background(), keyDailyTrades(), "1", 0).Err()
	if !CanTrade(10) {
		t.Fatal("quota left")
	}
}

func TestResetDailyStats_NoRedis(t *testing.T) {
	prev := redisx.Client
	redisx.Client = nil
	t.Cleanup(func() { redisx.Client = prev })
	if err := ResetDailyStats(); err == nil {
		t.Fatal("expected error")
	}
}

func TestResetDailyStats(t *testing.T) {
	testutil.UseMiniredis(t)
	t.Setenv("RPC_STUB", "1")
	if _, err := config.Load(); err != nil {
		t.Fatal(err)
	}
	_ = UpdateDailyLoss(-1)
	_ = IncrDailyTradeCount()
	if err := ResetDailyStats(); err != nil {
		t.Fatal(err)
	}
	loss, _ := DailyLossSOL()
	n, _ := DailyTradeCount()
	if loss != 0 || n != 0 {
		t.Fatalf("loss=%v n=%v", loss, n)
	}
}

func TestUpdateDailyLoss_NoRedis(t *testing.T) {
	prev := redisx.Client
	redisx.Client = nil
	t.Cleanup(func() { redisx.Client = prev })
	if err := UpdateDailyLoss(-1); err != nil {
		t.Fatal(err)
	}
}

func TestDailyLossSOL_NoRedis(t *testing.T) {
	prev := redisx.Client
	redisx.Client = nil
	t.Cleanup(func() { redisx.Client = prev })
	if _, err := DailyLossSOL(); err == nil {
		t.Fatal("expected error")
	}
}

func TestIncrDailyTradeCount_NoRedis(t *testing.T) {
	prev := redisx.Client
	redisx.Client = nil
	t.Cleanup(func() { redisx.Client = prev })
	if err := IncrDailyTradeCount(); err == nil {
		t.Fatal("expected error")
	}
}
