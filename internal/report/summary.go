package report

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"rlangga/internal/aggregate"
	"rlangga/internal/config"
	"rlangga/internal/log"
	"rlangga/internal/redisx"
	"rlangga/internal/store"
)

const (
	keyReportCount    = "report:trade_count"
	keyReportLastSent = "report:last_sent_unix"
)

// ResetReportState mengosongkan counter laporan periodik ([REPORT] tick / full summary).
func ResetReportState(ctx context.Context) error {
	if redisx.Client == nil {
		return fmt.Errorf("report: redis not initialized")
	}
	return redisx.Client.Del(ctx, keyReportCount, keyReportLastSent).Err()
}

// telegramAPIBase is the API root (override in tests).
var telegramAPIBase = "https://api.telegram.org"

// SetTelegramAPIBase mengganti root API Telegram (utama untuk tes).
func SetTelegramAPIBase(base string) (restore func()) {
	prev := telegramAPIBase
	telegramAPIBase = base
	return func() { telegramAPIBase = prev }
}

// SendSummary formats stats, writes the same text to stdout ([REPORT]), then sends to Telegram if configured (PR-003).
func SendSummary(s aggregate.Stats, streak int) error {
	cfg := config.C
	if cfg == nil {
		return nil
	}
	msg := fmt.Sprintf(`RLANGGA REPORT

Trades: %d
Winrate: %.2f%%
Total PnL: %.4f SOL
Avg: %.4f SOL
Loss Streak: %d
`, s.Total, s.Winrate, s.TotalPnL, s.AvgPnL, streak)
	log.Info("[REPORT]\n" + msg)

	if cfg.TelegramBotToken == "" || cfg.TelegramChatID == "" {
		return nil
	}

	u := fmt.Sprintf("%s/bot%s/sendMessage", telegramAPIBase, cfg.TelegramBotToken)
	body, _ := json.Marshal(map[string]string{
		"chat_id": cfg.TelegramChatID,
		"text":    msg,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram: HTTP %s", resp.Status)
	}
	return nil
}

// SendPlainMessage logs [ALERT] ke stdout, lalu Telegram jika token & chat terkonfigurasi (PR-005, dll.).
func SendPlainMessage(text string) error {
	cfg := config.C
	if cfg == nil {
		return nil
	}
	log.Info("[ALERT] " + text)
	if cfg.TelegramBotToken == "" || cfg.TelegramChatID == "" {
		return nil
	}
	u := fmt.Sprintf("%s/bot%s/sendMessage", telegramAPIBase, cfg.TelegramBotToken)
	body, _ := json.Marshal(map[string]string{
		"chat_id": cfg.TelegramChatID,
		"text":    text,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram: HTTP %s", resp.Status)
	}
	return nil
}

// LogTradeRealtime writes one line per closed trade to stdout (real-time), before batch [REPORT] summary.
func LogTradeRealtime(t store.Trade, saved bool, saveErr error) {
	if saveErr != nil {
		log.Info(fmt.Sprintf("[TRADE] mint=%s save_error: %v", t.Mint, saveErr))
		return
	}
	if !saved {
		log.Info(fmt.Sprintf("[TRADE] mint=%s bot=%s duplicate_skip (dedupe)", t.Mint, t.BotName))
		return
	}
	times := tradeTimeLogSuffix(t)
	entry := tradeEntryLogSuffix(t)
	if t.ExitReason != "" {
		log.Info(fmt.Sprintf("[TRADE] mint=%s bot=%s exit=%s%s%s pnl_sol=%.6f pct=%.2f%% dur_s=%d",
			t.Mint, t.BotName, t.ExitReason, times, entry, t.PnLSOL, t.Percent, t.DurationSec))
	} else {
		log.Info(fmt.Sprintf("[TRADE] mint=%s bot=%s%s%s pnl_sol=%.6f pct=%.2f%% dur_s=%d",
			t.Mint, t.BotName, times, entry, t.PnLSOL, t.Percent, t.DurationSec))
	}
}

func tradeTimeLogSuffix(t store.Trade) string {
	var parts []string
	if t.BuyTS != 0 {
		parts = append(parts, fmt.Sprintf("buy_ts=%d", t.BuyTS))
	}
	if t.TS != 0 {
		parts = append(parts, fmt.Sprintf("sell_ts=%d", t.TS))
	}
	if t.EntryStreamTimestampMs != 0 {
		parts = append(parts, fmt.Sprintf("wss_ts_ms=%d", t.EntryStreamTimestampMs))
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ")
}

func tradeEntryLogSuffix(t store.Trade) string {
	if t.EntryPool == "" && t.EntryPoolID == "" && t.EntryInitialBuy == 0 && t.EntryMarketCapSOL == 0 {
		return ""
	}
	return fmt.Sprintf(" entry_ib=%.6g mcap_sol=%.4f pool=%s",
		t.EntryInitialBuy, t.EntryMarketCapSOL, t.EntryPool)
}

// NotifyTradeSaved increments counters and may send a summary when thresholds are met (after each SaveTrade).
func NotifyTradeSaved() error {
	if redisx.Client == nil || config.C == nil {
		return nil
	}
	cfg := config.C
	ctx := context.Background()

	n, err := redisx.Client.Incr(ctx, keyReportCount).Result()
	if err != nil {
		return err
	}
	log.Info(fmt.Sprintf("[REPORT] tick since_last_full=%d (every_n=%d interval_min=%d)",
		n, cfg.ReportEveryNTrades, cfg.ReportIntervalMin))
	now := time.Now().Unix()
	v, err := redisx.Client.Get(ctx, keyReportLastSent).Result()
	var last int64
	switch err {
	case redis.Nil:
		last = 0
	case nil:
		last, _ = strconv.ParseInt(v, 10, 64)
	default:
		return err
	}
	if last == 0 {
		_ = redisx.Client.Set(ctx, keyReportLastSent, now, 0).Err()
	}

	should := false
	if cfg.ReportEveryNTrades > 0 && n >= int64(cfg.ReportEveryNTrades) {
		should = true
	}
	if cfg.ReportIntervalMin > 0 && last > 0 && now-last >= int64(cfg.ReportIntervalMin*60) {
		should = true
	}
	if !should {
		return nil
	}

	limit := cfg.ReportMaxTrades
	if limit <= 0 {
		limit = 200
	}
	trades, err := store.LoadRecent(limit)
	if err != nil {
		return err
	}
	st := aggregate.ComputeStats(trades)
	streak := aggregate.LossStreak(trades)
	if err := SendSummary(st, streak); err != nil {
		return err
	}
	_ = redisx.Client.Set(ctx, keyReportCount, 0, 0).Err()
	_ = redisx.Client.Set(ctx, keyReportLastSent, now, 0).Err()
	return nil
}
