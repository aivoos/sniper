package store

import (
	"database/sql"
	"path/filepath"
	"strings"

	"rlangga/internal/log"

	_ "github.com/glebarez/go-sqlite"
)

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS trades (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	mint TEXT NOT NULL,
	bot_name TEXT,
	buy_sol REAL NOT NULL,
	sell_sol REAL NOT NULL,
	pnl_sol REAL NOT NULL,
	percent REAL NOT NULL,
	duration_sec INTEGER NOT NULL,
	exit_reason TEXT,
	ts INTEGER NOT NULL,
	buy_ts INTEGER,
	entry_initial_buy REAL,
	entry_market_cap_sol REAL,
	entry_pool TEXT,
	entry_pool_id TEXT,
	entry_stream_ts_ms INTEGER,
	entry_tx_type TEXT,
	entry_pool_created_by TEXT,
	entry_burned_liquidity_pct REAL,
	entry_sol_in_pool REAL,
	entry_tokens_in_pool REAL,
	entry_activity_recorded INTEGER,
	entry_buy_sell_ratio REAL,
	entry_mcap_rising INTEGER
);
CREATE INDEX IF NOT EXISTS idx_trades_ts ON trades(ts DESC);
CREATE INDEX IF NOT EXISTS idx_trades_exit_reason ON trades(exit_reason);
CREATE INDEX IF NOT EXISTS idx_trades_buy_ts ON trades(buy_ts DESC);
`

var sqliteDB *sql.DB

// InitTradeSQLite membuka SQLite (opsional). Kosong = tidak memakai SQL lokal.
func InitTradeSQLite(path string) error {
	p := strings.TrimSpace(path)
	if p == "" {
		return nil
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return err
	}
	dsn := "file:" + filepath.ToSlash(abs) + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(sqliteSchema); err != nil {
		_ = db.Close()
		return err
	}
	migrateSQLiteColumns(db)
	sqliteDB = db
	return nil
}

func migrateSQLiteColumns(db *sql.DB) {
	// DB lama tanpa kolom baru — ALTER sekali; abaikan duplicate column.
	alters := []string{
		`ALTER TABLE trades ADD COLUMN buy_ts INTEGER`,
		`ALTER TABLE trades ADD COLUMN entry_stream_ts_ms INTEGER`,
		`ALTER TABLE trades ADD COLUMN entry_tx_type TEXT`,
		`ALTER TABLE trades ADD COLUMN entry_pool_created_by TEXT`,
		`ALTER TABLE trades ADD COLUMN entry_burned_liquidity_pct REAL`,
		`ALTER TABLE trades ADD COLUMN entry_sol_in_pool REAL`,
		`ALTER TABLE trades ADD COLUMN entry_tokens_in_pool REAL`,
		`ALTER TABLE trades ADD COLUMN entry_activity_recorded INTEGER`,
		`ALTER TABLE trades ADD COLUMN entry_buy_sell_ratio REAL`,
		`ALTER TABLE trades ADD COLUMN entry_mcap_rising INTEGER`,
	}
	for _, q := range alters {
		if _, err := db.Exec(q); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			log.Error("store: sqlite migrate: " + err.Error())
		}
	}
}

func insertTradeSQLite(t Trade) {
	if sqliteDB == nil {
		return
	}
	const q = `INSERT INTO trades (
		mint, bot_name, buy_sol, sell_sol, pnl_sol, percent, duration_sec, exit_reason, ts,
		buy_ts, entry_initial_buy, entry_market_cap_sol, entry_pool, entry_pool_id, entry_stream_ts_ms,
		entry_tx_type, entry_pool_created_by, entry_burned_liquidity_pct, entry_sol_in_pool, entry_tokens_in_pool,
		entry_activity_recorded, entry_buy_sell_ratio, entry_mcap_rising
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`
	_, err := sqliteDB.Exec(q,
		t.Mint, t.BotName, t.BuySOL, t.SellSOL, t.PnLSOL, t.Percent, t.DurationSec, nullIfEmpty(t.ExitReason), t.TS,
		nullInt64(t.BuyTS), t.EntryInitialBuy, t.EntryMarketCapSOL, nullIfEmpty(t.EntryPool), nullIfEmpty(t.EntryPoolID),
		nullInt64(t.EntryStreamTimestampMs),
		nullIfEmpty(t.EntryTxType), nullIfEmpty(t.EntryPoolCreatedBy), t.EntryBurnedLiquidityPct, t.EntrySolInPool, t.EntryTokensInPool,
		sqliteActivityRecorded(t.EntryActivityRecorded), sqliteRatioAtEntry(t.EntryActivityRecorded, t.EntryBuySellRatio), sqliteMcapRisingAtEntry(t.EntryActivityRecorded, t.EntryMcapRising),
	)
	if err != nil {
		log.Error("store: sqlite insert: " + err.Error())
	}
}

func nullIfEmpty(s string) interface{} {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func nullInt64(n int64) interface{} {
	if n == 0 {
		return nil
	}
	return n
}

func sqliteActivityRecorded(recorded bool) interface{} {
	if !recorded {
		return nil
	}
	return 1
}

func sqliteRatioAtEntry(recorded bool, ratio float64) interface{} {
	if !recorded {
		return nil
	}
	return ratio
}

func sqliteMcapRisingAtEntry(recorded bool, rising bool) interface{} {
	if !recorded {
		return nil
	}
	if rising {
		return 1
	}
	return 0
}
