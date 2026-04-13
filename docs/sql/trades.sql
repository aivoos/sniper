-- Skema trade tertutup (mirror dari Redis + kolom snapshot entry untuk tuning).
-- Implementasi runtime: SQLite via TRADE_SQLITE_PATH (internal/store/sqlite.go).
-- Untuk PostgreSQL / BigQuery: sesuaikan tipe (REAL → DOUBLE PRECISION, TEXT tetap).

-- --- SQLite / Postgres umum ---
CREATE TABLE IF NOT EXISTS trades (
	id INTEGER PRIMARY KEY AUTOINCREMENT, -- Postgres: SERIAL atau BIGSERIAL
	mint TEXT NOT NULL,
	bot_name TEXT,
	buy_sol REAL NOT NULL,
	sell_sol REAL NOT NULL,
	pnl_sol REAL NOT NULL,
	percent REAL NOT NULL,
	duration_sec INTEGER NOT NULL,
	exit_reason TEXT,
	ts INTEGER NOT NULL,          -- unix detik — tutup posisi / SELL terekam (sell_ts)
	buy_ts INTEGER,               -- unix detik — buka posisi (setelah BUY, mulai monitor)
	-- Snapshot pra-BUY dari payload WebSocket (PumpAPI create, dll.):
	entry_initial_buy REAL,       -- field JSON initialBuy (unit token)
	entry_market_cap_sol REAL,    -- proxy ukuran dari stream (bukan reserve LP on-chain)
entry_pool TEXT,
entry_pool_id TEXT,
entry_stream_ts_ms INTEGER,   -- field "timestamp" dari WSS (ms), jika ada
entry_tx_type TEXT,           -- txType di payload (buy/sell/add/remove/...)
entry_pool_created_by TEXT,   -- pump (migration) atau custom (manual), dll.
entry_burned_liquidity_pct REAL, -- parse dari burnedLiquidity (0-100)
entry_sol_in_pool REAL,       -- reserve SOL di pool (jika quote SOL)
entry_tokens_in_pool REAL     -- reserve token di pool
);

CREATE INDEX IF NOT EXISTS idx_trades_ts ON trades(ts DESC);
CREATE INDEX IF NOT EXISTS idx_trades_buy_ts ON trades(buy_ts DESC);
CREATE INDEX IF NOT EXISTS idx_trades_exit_reason ON trades(exit_reason);

-- --- Contoh analisis tuning ---
-- Win rate per alasan exit:
-- SELECT exit_reason, COUNT(*) AS n,
--        SUM(CASE WHEN pnl_sol > 0 THEN 1 ELSE 0 END) * 100.0 / COUNT(*) AS win_pct,
--        AVG(pnl_sol) AS avg_pnl_sol
-- FROM trades
-- GROUP BY exit_reason
-- ORDER BY n DESC;

-- Performa vs likuiditas proxy di entry (kuartil market cap):
-- SELECT
--   CASE
--     WHEN entry_market_cap_sol IS NULL OR entry_market_cap_sol <= 0 THEN 'unknown'
--     WHEN entry_market_cap_sol < 30 THEN 'mcap_lt_30'
--     WHEN entry_market_cap_sol < 60 THEN 'mcap_30_60'
--     ELSE 'mcap_ge_60'
--   END AS bucket,
--   COUNT(*) AS n, AVG(pnl_sol) AS avg_pnl
-- FROM trades
-- GROUP BY 1;

-- Initial buy vs hasil (token amount dari stream — skala sama per venue):
-- SELECT AVG(pnl_sol) AS avg_pnl
-- FROM trades
-- WHERE entry_initial_buy >= 50000000;
