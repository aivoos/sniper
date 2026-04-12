package report

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"rlangga/internal/aggregate"
	"rlangga/internal/config"
	"rlangga/internal/redisx"
	"rlangga/internal/store"
)

const (
	keyReportCount    = "report:trade_count"
	keyReportLastSent = "report:last_sent_unix"
)

// telegramAPIBase is the API root (override in tests).
var telegramAPIBase = "https://api.telegram.org"

// SendSummary formats stats and sends to Telegram when bot token and chat ID are set (PR-003).
func SendSummary(s aggregate.Stats, streak int) error {
	cfg := config.C
	if cfg == nil {
		return nil
	}
	if cfg.TelegramBotToken == "" || cfg.TelegramChatID == "" {
		return nil
	}
	msg := fmt.Sprintf(`RLANGGA REPORT

Trades: %d
Winrate: %.2f%%
Total PnL: %.4f SOL
Avg: %.4f SOL
Loss Streak: %d
`, s.Total, s.Winrate, s.TotalPnL, s.AvgPnL, streak)

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
