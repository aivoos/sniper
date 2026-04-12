package guard

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"rlangga/internal/config"
	"rlangga/internal/redisx"
	"rlangga/internal/report"
)

// Redis key prefix — harian per UTC (rollover otomatis). Dok PR-003 menyebut stats:daily_loss; sufiks tanggal menghindari loss menggantung tanpa cron.
const (
	prefixDailyLoss   = "stats:daily_loss:"
	prefixDailyTrades = "stats:daily_trades:"
	prefixKSAlert     = "guard:kill_switch_alert:"
)

func todayUTC() string {
	return time.Now().UTC().Format("2006-01-02")
}

func keyDailyLoss() string   { return prefixDailyLoss + todayUTC() }
func keyDailyTrades() string { return prefixDailyTrades + todayUTC() }
func keyKSAlert() string     { return prefixKSAlert + todayUTC() }

// UpdateDailyLoss menambah akumulasi rugi harian setelah trade tertutup rugi (PR-005).
func UpdateDailyLoss(pnlSOL float64) error {
	if pnlSOL >= 0 || redisx.Client == nil {
		return nil
	}
	ctx := context.Background()
	_, err := redisx.Client.IncrByFloat(ctx, keyDailyLoss(), -pnlSOL).Result()
	return err
}

// DailyLossSOL membaca total rugi terakumulasi (SOL) untuk hari UTC berjalan.
func DailyLossSOL() (float64, error) {
	if redisx.Client == nil {
		return 0, fmt.Errorf("guard: redis not initialized")
	}
	ctx := context.Background()
	v, err := redisx.Client.Get(ctx, keyDailyLoss()).Float64()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return v, nil
}

// IncrDailyTradeCount menambah counter BUY sukses untuk kuota harian (setelah BuyAndValidate OK).
func IncrDailyTradeCount() error {
	if redisx.Client == nil {
		return fmt.Errorf("guard: redis not initialized")
	}
	ctx := context.Background()
	return redisx.Client.Incr(ctx, keyDailyTrades()).Err()
}

// DailyTradeCount membaca jumlah BUY sukses hari ini (UTC).
func DailyTradeCount() (int64, error) {
	if redisx.Client == nil {
		return 0, fmt.Errorf("guard: redis not initialized")
	}
	ctx := context.Background()
	v, err := redisx.Client.Get(ctx, keyDailyTrades()).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(v, 10, 64)
}

// IsKillSwitchTriggered membandingkan akumulasi rugi dengan MAX_DAILY_LOSS (0 = nonaktif).
func IsKillSwitchTriggered() bool {
	cfg := config.C
	if cfg == nil || cfg.MaxDailyLoss <= 0 {
		return false
	}
	loss, err := DailyLossSOL()
	if err != nil {
		return false
	}
	return loss >= cfg.MaxDailyLoss
}

// HasEnoughBalance memeriksa saldo vs MIN_BALANCE.
func HasEnoughBalance(balanceSOL float64) bool {
	cfg := config.C
	if cfg == nil {
		return false
	}
	return balanceSOL >= cfg.MinBalance
}

// CanTrade gate untuk BUY baru saja — recovery / SELL tidak memanggil ini (PR-005).
func CanTrade(balanceSOL float64) bool {
	cfg := config.C
	if cfg == nil || !cfg.EnableTrading {
		return false
	}
	if IsKillSwitchTriggered() {
		_ = maybeSendKillSwitchAlert()
		return false
	}
	if !HasEnoughBalance(balanceSOL) {
		return false
	}
	if cfg.MaxDailyTrades > 0 {
		n, err := DailyTradeCount()
		if err == nil && n >= int64(cfg.MaxDailyTrades) {
			return false
		}
	}
	return true
}

func maybeSendKillSwitchAlert() error {
	cfg := config.C
	if cfg == nil || redisx.Client == nil {
		return nil
	}
	loss, err := DailyLossSOL()
	if err != nil {
		return nil
	}
	ctx := context.Background()
	ok, err := redisx.Client.SetNX(ctx, keyKSAlert(), "1", 26*time.Hour).Result()
	if err != nil || !ok {
		return err
	}
	msg := fmt.Sprintf("RLANGGA KILL SWITCH\nLoss: %.4f SOL (max %.4f)\nTrading stopped for new BUYs", loss, cfg.MaxDailyLoss)
	return report.SendPlainMessage(msg)
}

// ResetDailyStats mengenolkan loss dan counter trade untuk hari UTC berjalan (cron / manual).
func ResetDailyStats() error {
	if redisx.Client == nil {
		return fmt.Errorf("guard: redis not initialized")
	}
	ctx := context.Background()
	pipe := redisx.Client.Pipeline()
	pipe.Set(ctx, keyDailyLoss(), "0", 0)
	pipe.Set(ctx, keyDailyTrades(), "0", 0)
	_, err := pipe.Exec(ctx)
	return err
}
