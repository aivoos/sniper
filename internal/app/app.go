package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"rlangga/internal/bot"
	"rlangga/internal/config"
	"rlangga/internal/executor"
	"rlangga/internal/filter"
	"rlangga/internal/guard"
	"rlangga/internal/idempotency"
	"rlangga/internal/lock"
	"rlangga/internal/log"
	"rlangga/internal/monitor"
	"rlangga/internal/orchestrator"
	"rlangga/internal/pumpws"
	"rlangga/internal/redisx"
	"rlangga/internal/store"
	"rlangga/internal/wallet"
)

// Init loads config, connects Redis, and installs multi-bot profiles (PR-004).
func Init() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if cfg.RedisURL == "" {
		return errors.New("REDIS_URL is required")
	}
	if err := redisx.Init(cfg.RedisURL); err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	if err := store.InitTradeSQLite(cfg.TradeSQLitePath); err != nil {
		return fmt.Errorf("trade sqlite: %w", err)
	}
	if strings.TrimSpace(cfg.TradeSQLitePath) != "" {
		fmt.Println("RLANGGA TRADE_SQLITE: mirror trade tertutup → " + cfg.TradeSQLitePath + " (query SQL untuk tuning)")
	}
	profiles, err := bot.LoadBots()
	if err != nil {
		return fmt.Errorf("bots: %w", err)
	}
	orchestrator.Init(profiles)
	if cfg.PaperTrade {
		fmt.Println("RLANGGA PAPER_TRADE: RPC mainnet (Helius dari kunci atau RPC_URL); sesuaikan Pump/gateway dengan jaringan Anda")
	}
	if cfg.SimulateEngine {
		fmt.Println("RLANGGA SIMULATE_ENGINE: mesin exit adaptif + BUY/SELL tanpa transaksi (quote nyata jika terkonfigurasi; jika tidak, quote sintetis)")
		if cfg.SimulateSynthAmplitudePct > 0 || cfg.SimulateSynthPeriodSec > 0 || cfg.SimulateSynthDriftPct != 0 {
			fmt.Printf("RLANGGA SIMULATE_SYNTH: amplitude_pct=%.2f period_sec=%.2f drift_pct=%.2f (0=default amp/period)\n",
				cfg.SimulateSynthAmplitudePct, cfg.SimulateSynthPeriodSec, cfg.SimulateSynthDriftPct)
		}
	} else if cfg.SimulateTrading && strings.TrimSpace(cfg.PumpWSURL) != "" {
		fmt.Println("RLANGGA SIMULATE_TRADING: WebSocket stream aktif; HandleMint hanya log [SIM], tanpa eksekusi trade")
	}
	if !cfg.RPCStub && len(cfg.RPCURLs) > 1 {
		fmt.Printf("RLANGGA RPC: %d endpoints (failover getSignatureStatuses)\n", len(cfg.RPCURLs))
	}
	if cfg.FilterRequireInitialBuy {
		fmt.Println("RLANGGA FILTER_REQUIRE_INITIAL_BUY: wajib field initialBuy di payload WSS (snapshot stream)")
	}
	if cfg.FilterAntiRug {
		fmt.Println("RLANGGA FILTER_ANTI_RUG: gate on-chain sebelum BUY (freeze/mint authority, opsional konsentrasi holder)")
	}
	if cfg.FilterWSSGateActive() {
		fmt.Println("RLANGGA FILTER_WSS: gate dari payload WebSocket sebelum HandleMint (tx type / method / SOL / pool / market cap — lihat docs/wss-data-for-filters.md)")
	}
	if cfg.FilterMinInitialBuy > 0 || cfg.FilterMinEntryMarketCapSOL > 0 {
		fmt.Println("RLANGGA FILTER_ENTRY: minimum initialBuy / marketCapSol dari snapshot stream (hanya jika field ada di payload)")
	}
	fmt.Println("RLANGGA INIT")
	return nil
}

// HandleMint: PR-005 gate → filter RPC (opsional) → idempotency → lock → buy → adaptive monitor (PR-002).
// Filter dijalankan sebelum idempotency agar kegagalan filter tidak mengonsumsi dedupe Redis.
// entry: snapshot dari WebSocket (initialBuy, marketCapSol, pool) jika ParseStreamEvent berhasil; nil jika hanya mint dari ExtractMint.
func HandleMint(mint string, entry *pumpws.StreamEvent) {
	// Stream-only: log mint tanpa mesin (kecuali SIMULATE_ENGINE — jalur penuh di bawah).
	if config.C != nil && config.C.SimulateTrading && !config.C.SimulateEngine {
		if idempotency.IsDuplicate(mint) {
			return
		}
		log.Info(fmt.Sprintf("[SIM] stream mint=%s (SIMULATE_TRADING=1, no on-chain BUY)", mint))
		return
	}
	bal := wallet.GetSOLBalance()
	if !guard.CanTrade(bal) {
		log.Info("TRADE BLOCKED")
		return
	}
	if config.C != nil && config.C.FilterRequireInitialBuy {
		if entry == nil || !entry.HasInitialBuy {
			log.Info("FILTER: initialBuy required from WSS mint=" + mint)
			return
		}
	}
	if config.C != nil && config.C.FilterAntiRug {
		ok, reason := filter.AllowMint(context.Background(), mint)
		if !ok {
			log.Info("FILTER: " + reason + " mint=" + mint)
			return
		}
	}
	if config.C != nil && entry != nil && !entrySnapshotPasses(config.C, entry) {
		log.Info(fmt.Sprintf("FILTER_ENTRY: initial_buy/mcap below threshold mint=%s", mint))
		return
	}
	if idempotency.IsDuplicate(mint) {
		return
	}
	if !lock.LockMint(mint) {
		return
	}
	b := orchestrator.NextBot()
	if config.C != nil && config.C.SimulateEngine {
		log.Info(fmt.Sprintf("[SIM-ENGINE] [%s] paper position %s", b.Name, mint))
	} else {
		log.Info(fmt.Sprintf("[%s] BUY %s", b.Name, mint))
	}
	success := executor.BuyAndValidate(mint)
	if !success {
		lock.UnlockMint(mint)
		return
	}
	if err := guard.IncrDailyTradeCount(); err != nil {
		log.Info("guard: IncrDailyTradeCount: " + err.Error())
	}
	if config.C == nil {
		lock.UnlockMint(mint)
		return
	}
	buySOL := config.C.TradeSize
	monitor.MonitorPositionWithBot(mint, buySOL, b, entry)
	lock.UnlockMint(mint)
}

func entrySnapshotPasses(cfg *config.Config, ev *pumpws.StreamEvent) bool {
	if cfg.FilterMinInitialBuy > 0 && ev.HasInitialBuy && ev.InitialBuy < cfg.FilterMinInitialBuy {
		return false
	}
	if cfg.FilterMinEntryMarketCapSOL > 0 && ev.HasMarketCapSOL && ev.MarketCapSOL < cfg.FilterMinEntryMarketCapSOL {
		return false
	}
	if cfg.FilterRejectPoolCreatedByCustom && strings.TrimSpace(strings.ToLower(ev.PoolCreatedBy)) == "custom" {
		return false
	}
	if cfg.FilterMinBurnedLiquidityPct > 0 && ev.HasBurnedLiquidity && ev.BurnedLiquidityPct < cfg.FilterMinBurnedLiquidityPct {
		return false
	}
	if cfg.FilterMinEntrySolInPool > 0 && ev.HasSolInPool && ev.SolInPool < cfg.FilterMinEntrySolInPool {
		return false
	}
	return true
}

// startPumpStream memanggil pumpws.Run untuk stream primer + opsional fallback (paralel).
// Keduanya memakai dispatchStreamMint: satu keluaran logika per mint bersamaan (lihat stream_merge.go).
func startPumpStream(ctx context.Context, cfg *config.Config) {
	if cfg == nil {
		return
	}
	autoHandle := cfg.PumpWSAutoHandle
	// Satu callback untuk PUMP_WS_URL dan PUMP_WS_FALLBACK_URL: filter memakai StreamEvent yang sama (parse + nested field).
	onMsg := func(msg []byte) {
		if !autoHandle {
			return
		}
		ev, ok := pumpws.ParseStreamEvent(msg)
		if ok && ev.Mint != "" {
			// Broadcast to in-process subscribers (e.g. monitor rug exits).
			pumpws.PublishStreamEvent(ev)
			if cfg.FilterWSSGateActive() {
				if pass, reason := filter.AllowStreamEvent(&ev); !pass {
					log.Info("FILTER_WSS: " + reason + " mint=" + ev.Mint)
					return
				}
			}
			dispatchStreamMintEvent(ev)
			return
		}
		mint := pumpws.ExtractMint(msg)
		dispatchStreamMint(mint)
	}
	pumpws.Run(ctx, pumpws.Options{
		URL:           strings.TrimSpace(cfg.PumpWSURL),
		SubscribeJSON: cfg.PumpWSSubscribeJSON,
	}, onMsg)
	fbSub := cfg.PumpWSFallbackSubscribeJSON
	if fbSub == "" {
		fbSub = cfg.PumpWSSubscribeJSON
	}
	pumpws.Run(ctx, pumpws.Options{
		URL:           strings.TrimSpace(cfg.PumpWSFallbackURL),
		SubscribeJSON: fbSub,
	}, onMsg)
}

// StartWorker menjalankan listener WebSocket opsional (PUMP_WS_URL) dan memblok selamanya.
func StartWorker() {
	log.Info("Worker running (PR-002 adaptive exit + opsional Pump WS)")
	log.Info("report: [TRADE] / [REPORT] hanya ke stdout setelah SaveTrade sukses; idle = tidak ada baris laporan. Background: pakai tail -f pada file log.")
	startPumpStream(context.Background(), config.C)
	select {}
}
