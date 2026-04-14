package report

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"rlangga/internal/aggregate"
	"rlangga/internal/config"
	"rlangga/internal/log"
	"rlangga/internal/redisx"
	"rlangga/internal/store"
	"rlangga/internal/wallet"
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

// NotifyBotStarted sends a startup status report to Telegram with key config values.
func NotifyBotStarted() error {
	cfg := config.C
	if cfg == nil {
		return nil
	}
	mode := "LIVE"
	if cfg.SimulateEngine {
		mode = "SIMULATE"
	}
	if cfg.RPCStub {
		mode += " (RPC_STUB)"
	}

	bal := wallet.GetSOLBalance()
	tradeSize := wallet.GetTradeSize()

	var sb strings.Builder
	sb.WriteString("=== BOT STARTED ===\n\n")
	sb.WriteString(fmt.Sprintf("Mode     : %s\n", mode))
	sb.WriteString(fmt.Sprintf("Wallet   : %.4f SOL\n", bal))
	if cfg.TradeSizePct > 0 {
		line := fmt.Sprintf("Trade    : %.4f SOL (%.1f%% of wallet)", tradeSize, cfg.TradeSizePct)
		if cfg.MaxTradeSizeSOL > 0 {
			line += fmt.Sprintf(" [max %.4f SOL/BUY]", cfg.MaxTradeSizeSOL)
		}
		sb.WriteString(line + "\n")
	} else {
		line := fmt.Sprintf("Trade    : %.4f SOL (fixed)", tradeSize)
		if cfg.MaxTradeSizeSOL > 0 {
			line += fmt.Sprintf(" [max %.4f SOL/BUY]", cfg.MaxTradeSizeSOL)
		}
		sb.WriteString(line + "\n")
	}
	sb.WriteString(fmt.Sprintf("TP/SL    : +%.1f%% / -%.1f%%\n", cfg.TakeProfit, cfg.StopLoss))
	sb.WriteString(fmt.Sprintf("Panic SL : -%.1f%%\n", cfg.PanicSL))
	if cfg.GraceTP > 0 {
		if cfg.GraceTrailDrop > 0 {
			sb.WriteString(fmt.Sprintf("Grace TP : +%.1f%% trail -%.1f%% (%ds)\n", cfg.GraceTP, cfg.GraceTrailDrop, cfg.GraceSeconds))
		} else {
			sb.WriteString(fmt.Sprintf("Grace TP : +%.1f%% (%ds)\n", cfg.GraceTP, cfg.GraceSeconds))
		}
	}
	if cfg.GraceSL > 0 {
		sb.WriteString(fmt.Sprintf("Grace SL : -%.1f%% (%ds)\n", cfg.GraceSL, cfg.GraceSeconds))
	}
	sb.WriteString(fmt.Sprintf("Max Hold : %ds\n", cfg.MaxHold))
	if cfg.ConfirmSLMS > 0 {
		sb.WriteString(fmt.Sprintf("Anti-Wick: %dms\n", cfg.ConfirmSLMS))
	}
	if cfg.WhaleSellMinSOL > 0 {
		sb.WriteString(fmt.Sprintf("Whale Det: %.2f SOL\n", cfg.WhaleSellMinSOL))
	}

	sb.WriteString("\n--- Filters ---\n")
	if len(cfg.FilterWSSPoolAllow) > 0 {
		sb.WriteString(fmt.Sprintf("Pool     : %s\n", strings.Join(cfg.FilterWSSPoolAllow, ", ")))
	}
	if cfg.FilterWSSMinMarketCapSOL > 0 {
		sb.WriteString(fmt.Sprintf("Min MCap : %.2f SOL\n", cfg.FilterWSSMinMarketCapSOL))
	}
	if cfg.FilterWSSMinSolInPool > 0 {
		sb.WriteString(fmt.Sprintf("Min Pool : %.2f SOL\n", cfg.FilterWSSMinSolInPool))
	}
	if cfg.FilterMinBuySellRatio > 0 {
		sb.WriteString(fmt.Sprintf("B/S Ratio: >= %.1f\n", cfg.FilterMinBuySellRatio))
	}
	if cfg.FilterRequireMcapRise {
		sb.WriteString("MCap Rise: required\n")
	}
	if cfg.ReportIntervalMin > 0 {
		sb.WriteString(fmt.Sprintf("\nReport   : every %d min (accumulated)\n", cfg.ReportIntervalMin))
	}

	msg := sb.String()
	log.Info("[STARTUP]\n" + msg)
	log.Info("[STARTUP] sending to telegram...")
	err := sendTelegram(msg)
	if err != nil {
		log.Info("[STARTUP] telegram FAILED: " + err.Error())
	} else {
		log.Info("[STARTUP] telegram OK")
	}
	return err
}

// SendSummary formats rich stats with exit breakdown, wallet balance, and top/worst trade.
func SendSummary(s aggregate.Stats, streak int) error {
	return SendSummaryWithTrades(s, streak, nil)
}

// SendSummaryWithTrades sends a rich summary including exit breakdown and top/worst trade from the provided trades slice.
func SendSummaryWithTrades(s aggregate.Stats, streak int, trades []store.Trade) error {
	cfg := config.C
	if cfg == nil {
		return nil
	}
	startBal := wallet.GetSOLBalance()
	currentBal := startBal + s.TotalPnL
	bankrollPct := 0.0
	if startBal > 0 {
		bankrollPct = (s.TotalPnL / startBal) * 100
	}
	var compoundTrade float64
	if cfg.TradeSizePct > 0 {
		raw := currentBal * cfg.TradeSizePct / 100
		compoundTrade = raw
		if cfg.MaxTradeSizeSOL > 0 && raw > cfg.MaxTradeSizeSOL {
			compoundTrade = cfg.MaxTradeSizeSOL
		}
	} else {
		compoundTrade = cfg.TradeSize
		if cfg.MaxTradeSizeSOL > 0 && compoundTrade > cfg.MaxTradeSizeSOL {
			compoundTrade = cfg.MaxTradeSizeSOL
		}
	}
	maxDD := computeMaxDrawdown(trades)

	now := time.Now()
	avgPnL := 0.0
	if s.Total > 0 {
		avgPnL = s.TotalPnL / float64(s.Total)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== RLANGGA REPORT === %s\n\n", now.Format("15:04:05")))

	sb.WriteString(fmt.Sprintf("Trades      : %d\n", s.Total))
	sb.WriteString(fmt.Sprintf("Win Rate    : %.2f%%\n", s.Winrate))
	sb.WriteString(fmt.Sprintf("Total PnL   : %+.4f SOL\n", s.TotalPnL))
	sb.WriteString(fmt.Sprintf("Avg PnL     : %+.4f SOL\n", avgPnL))
	sb.WriteString(fmt.Sprintf("Bankroll    : %.4f SOL (%+.2f%%)\n", currentBal, bankrollPct))
	sb.WriteString(fmt.Sprintf("Next Buy    : %.4f SOL\n", compoundTrade))
	sb.WriteString(fmt.Sprintf("Max Drawdown: %.2f%%\n", maxDD))
	sb.WriteString(fmt.Sprintf("Loss Streak : %d\n", streak))

	if len(trades) > 0 {
		sb.WriteString("\n--- Exit Breakdown ---\n")
		sb.WriteString(fmt.Sprintf("%-12s %5s %5s %9s %9s\n", "Exit", "Count", "Win%", "TotalPnL", "AvgPnL"))
		sb.WriteString(formatExitBreakdown(trades))

		best, worst := topWorstTrade(trades)
		if best != nil {
			sb.WriteString(fmt.Sprintf("\nBest : %+.4f SOL (%.1f%%) %s [%s]\n", best.PnLSOL, best.Percent, shortMint(best.Mint), best.ExitReason))
		}
		if worst != nil {
			sb.WriteString(fmt.Sprintf("Worst: %+.4f SOL (%.1f%%) %s [%s]\n", worst.PnLSOL, worst.Percent, shortMint(worst.Mint), worst.ExitReason))
		}

		last := trades[0]
		sb.WriteString(fmt.Sprintf("\nLast : %+.4f SOL (%.1f%%) %s [%s] %ds ago\n",
			last.PnLSOL, last.Percent, shortMint(last.Mint), last.ExitReason,
			int(now.Unix()-last.TS)))
	}

	msg := sb.String()
	log.Info("[REPORT]\n" + msg)

	return sendTelegram(msg)
}

func computeMaxDrawdown(trades []store.Trade) float64 {
	if len(trades) == 0 {
		return 0
	}
	cumPnL := 0.0
	peak := 0.0
	maxDD := 0.0
	for _, t := range trades {
		cumPnL += t.PnLSOL
		if cumPnL > peak {
			peak = cumPnL
		}
		dd := peak - cumPnL
		if dd > maxDD {
			maxDD = dd
		}
	}
	if peak <= 0 {
		return 0
	}
	return (maxDD / peak) * 100
}

// NotifyTradeCompleted sends a per-trade notification to Telegram.
func NotifyTradeCompleted(t store.Trade) error {
	cfg := config.C
	if cfg == nil || cfg.TelegramBotToken == "" || cfg.TelegramChatID == "" {
		return nil
	}
	icon := "+"
	if t.PnLSOL < 0 {
		icon = "-"
	}
	if t.PnLSOL >= 0 {
		icon = "+"
	}
	exit := t.ExitReason
	if exit == "" {
		exit = "unknown"
	}
	msg := fmt.Sprintf("[%s] %s%s | %+.4f SOL (%.1f%%) | %ds | %s",
		exit, icon, shortMint(t.Mint), t.PnLSOL, t.Percent, t.DurationSec, t.EntryPool)
	return sendTelegram(msg)
}

// NotifyAlert sends an alert for significant events (panic, rug, whale dump with large loss).
func NotifyAlert(t store.Trade) error {
	cfg := config.C
	if cfg == nil || cfg.TelegramBotToken == "" || cfg.TelegramChatID == "" {
		return nil
	}
	msg := fmt.Sprintf("ALERT %s\nMint: %s\nLoss: %+.4f SOL (%.1f%%)\nPool: %s | Dur: %ds",
		strings.ToUpper(t.ExitReason), t.Mint, t.PnLSOL, t.Percent, t.EntryPool, t.DurationSec)
	return sendTelegram(msg)
}

func sendTelegram(text string) error {
	cfg := config.C
	if cfg == nil || cfg.TelegramBotToken == "" || cfg.TelegramChatID == "" {
		return nil
	}

	const maxRetries = 2
	const retryDelay = 2 * time.Second
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			log.Info(fmt.Sprintf("telegram: retry %d/%d after %v", attempt, maxRetries, retryDelay))
			time.Sleep(retryDelay)
		}
		lastErr = sendTelegramOnce(cfg, text)
		if lastErr == nil {
			return nil
		}
		if !isRetryable(lastErr) {
			return lastErr
		}
	}
	return fmt.Errorf("telegram: all %d retries failed: %w", maxRetries, lastErr)
}

func sendTelegramOnce(cfg *config.Config, text string) error {
	u := fmt.Sprintf("%s/bot%s/sendMessage", telegramAPIBase, cfg.TelegramBotToken)
	body, _ := json.Marshal(map[string]string{
		"chat_id":    cfg.TelegramChatID,
		"text":       text,
		"parse_mode": "",
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
		log.Error("telegram: request error: " + err.Error())
		return &retryableError{err}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		var respBody bytes.Buffer
		respBody.ReadFrom(resp.Body)
		log.Error(fmt.Sprintf("telegram: HTTP %s body=%s", resp.Status, respBody.String()))
		return &retryableError{fmt.Errorf("telegram: HTTP %s", resp.Status)}
	}
	if resp.StatusCode >= 300 {
		var respBody bytes.Buffer
		respBody.ReadFrom(resp.Body)
		log.Error(fmt.Sprintf("telegram: HTTP %s body=%s", resp.Status, respBody.String()))
		return fmt.Errorf("telegram: HTTP %s", resp.Status)
	}
	return nil
}

type retryableError struct{ err error }

func (e *retryableError) Error() string { return e.err.Error() }
func (e *retryableError) Unwrap() error { return e.err }

func isRetryable(err error) bool {
	_, ok := err.(*retryableError)
	return ok
}

type exitAgg struct {
	reason string
	n      int
	wins   int
	pnl    float64
}

func formatExitBreakdown(trades []store.Trade) string {
	m := map[string]*exitAgg{}
	for _, t := range trades {
		r := t.ExitReason
		if r == "" {
			r = "other"
		}
		a := m[r]
		if a == nil {
			a = &exitAgg{reason: r}
			m[r] = a
		}
		a.n++
		a.pnl += t.PnLSOL
		if t.PnLSOL > 0 {
			a.wins++
		}
	}
	rows := make([]exitAgg, 0, len(m))
	for _, v := range m {
		rows = append(rows, *v)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].pnl > rows[j].pnl })

	var sb strings.Builder
	for _, r := range rows {
		wr := 0.0
		avg := 0.0
		if r.n > 0 {
			wr = float64(r.wins) / float64(r.n) * 100
			avg = r.pnl / float64(r.n)
		}
		sb.WriteString(fmt.Sprintf("%-12s %5d %4.0f%% %+9.4f %+9.4f\n", r.reason, r.n, wr, r.pnl, avg))
	}
	return sb.String()
}

func topWorstTrade(trades []store.Trade) (best *store.Trade, worst *store.Trade) {
	for i := range trades {
		if best == nil || trades[i].PnLSOL > best.PnLSOL {
			best = &trades[i]
		}
		if worst == nil || trades[i].PnLSOL < worst.PnLSOL {
			worst = &trades[i]
		}
	}
	return
}

func shortMint(mint string) string {
	if len(mint) <= 8 {
		return mint
	}
	return mint[:4] + ".." + mint[len(mint)-4:]
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

// NotifyTradeSaved sends per-trade notification, alerts for big losses, and periodic rich summary.
func NotifyTradeSaved() error {
	return NotifyTradeSavedWithTrade(store.Trade{})
}

// NotifyTradeSavedWithTrade sends per-trade notification + periodic summary.
func NotifyTradeSavedWithTrade(lastTrade store.Trade) error {
	if redisx.Client == nil || config.C == nil {
		return nil
	}
	cfg := config.C

	// Alert only for rug pulls and whale dumps (rare, critical events — no spam).
	if lastTrade.Mint != "" {
		isRug := lastTrade.ExitReason == "rug_remove_liquidity"
		isWhale := lastTrade.ExitReason == "whale_dump" && lastTrade.PnLSOL < -0.01
		if isRug || isWhale {
			if err := NotifyAlert(lastTrade); err != nil {
				log.Info("report: alert notify: " + err.Error())
			}
		}
	}

	ctx := context.Background()

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
		return nil
	}

	intervalSec := int64(cfg.ReportIntervalMin * 60)
	if intervalSec <= 0 {
		return nil
	}
	if now-last < intervalSec {
		return nil
	}

	limit := cfg.ReportMaxTrades
	trades, err := store.LoadTradesForReport(limit)
	if err != nil {
		return err
	}
	if len(trades) == 0 {
		return nil
	}
	st := aggregate.ComputeStats(trades)
	streak := aggregate.LossStreak(trades)
	if err := SendSummaryWithTrades(st, streak, trades); err != nil {
		return err
	}
	_ = redisx.Client.Set(ctx, keyReportLastSent, now, 0).Err()
	return nil
}

// StartPeriodicReport runs a background loop that sends summary reports
// every REPORT_INTERVAL_MIN minutes, independent of trade activity.
func StartPeriodicReport(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Error(fmt.Sprintf("PANIC RECOVERED [periodic-report]: %v", r))
		}
	}()
	cfg := config.C
	if cfg == nil {
		log.Info("report: periodic aborted (config nil)")
		return
	}
	if cfg.ReportIntervalMin <= 0 {
		log.Info("report: periodic aborted (interval <= 0)")
		return
	}
	interval := time.Duration(cfg.ReportIntervalMin) * time.Minute
	log.Info(fmt.Sprintf("report: periodic ticker started (every %v)", interval))

	for {
		select {
		case <-ctx.Done():
			log.Info("report: periodic stopped (ctx done)")
			return
		case <-time.After(interval):
			log.Info("report: periodic tick fired")
			sendPeriodicSnapshot()
		}
	}
}

func sendPeriodicSnapshot() {
	cfg := config.C
	if cfg == nil || redisx.Client == nil {
		log.Info("report: periodic skip (cfg/redis nil)")
		return
	}
	limit := cfg.ReportMaxTrades
	trades, err := store.LoadTradesForReport(limit)
	if err != nil {
		log.Error("report: periodic load: " + err.Error())
		return
	}
	log.Info(fmt.Sprintf("report: periodic snapshot (%d trades)", len(trades)))
	if len(trades) == 0 {
		if err := sendTelegram("SNAPSHOT: 0 trades (idle)"); err != nil {
			log.Error("report: periodic send: " + err.Error())
		}
		return
	}
	st := aggregate.ComputeStats(trades)
	streak := aggregate.LossStreak(trades)
	if err := SendSummaryWithTrades(st, streak, trades); err != nil {
		log.Error("report: periodic send: " + err.Error())
	}
}
