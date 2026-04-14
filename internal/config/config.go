package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
)

// C is populated by Load and read across packages (PR-001 + PR-002).
var C *Config

var validate = validator.New()

// Config holds environment for worker, exit engine, and recovery.
type Config struct {
	// Env-backed (see rlangga-env-contract.md). RedisURL checked at app.Init (worker), not here — tests load partial env.
	RedisURL      string   `validate:"omitempty"`
	RPCURL        string   `validate:"omitempty,url"`      // endpoint pertama (kompatibel); gunakan RPC_URLS untuk failover
	RPCURLs       []string `validate:"omitempty,dive,url"` // RPC_URLS dipisah koma, atau satu elemen dari RPC_URL
	PumpPortalURL string   `validate:"omitempty,url"`
	PumpAPIURL    string   `validate:"omitempty,url"`
	// Pump Lightning / PumpAPI JSON (opsional; lihat pumpnative).
	PumpNative                bool    // PUMP_NATIVE — aktifkan API resmi (trade + kunci).
	PumpPortalAPIKey          string  `validate:"omitempty"`
	PumpPortalQuoteURL        string  `validate:"omitempty,url"` // basis /quote jika beda dari PUMPPORTAL_URL
	PumpAPIQuoteURL           string  `validate:"omitempty,url"`
	WalletPublicKey           string  `validate:"omitempty"` // PUMP_WALLET_PUBLIC_KEY — fallback PumpAPI jika tanpa private key
	PumpPrivateKey            string  `validate:"omitempty"` // PUMP_PRIVATE_KEY — base58; disarankan untuk PumpAPI (field privateKey)
	PumpAPIQuoteMint          string  `validate:"omitempty"` // PUMPAPI_QUOTE_MINT — pool non-SOL quote
	PumpAPIPoolID             string  `validate:"omitempty"` // PUMPAPI_POOL_ID
	PumpAPIGuaranteedDelivery bool    // PUMPAPI_GUARANTEED_DELIVERY — rebroadcast hingga ~10s
	PumpAPIJitoTip            float64 `validate:"gte=0"` // PUMPAPI_JITO_TIP SOL
	PumpAPIMaxPriorityFee     float64 `validate:"gte=0"` // PUMPAPI_MAX_PRIORITY_FEE — cap untuk mode auto
	PumpAPIPriorityFeeMode    string  // PUMPAPI_PRIORITY_FEE_MODE — auto, auto-95, auto-75, auto-50 (override angka)
	PumpSlippage              float64 `validate:"gte=0"` // PUMP_SLIPPAGE — Portal Lightning (default 10)
	PumpAPISlippage           float64 `validate:"gte=0"` // PUMPAPI_SLIPPAGE — PumpAPI (default 20)
	PumpPriorityFee           float64 `validate:"gte=0"` // PUMP_PRIORITY_FEE SOL
	// Estimated protocol fee (percent). Used for net PnL calculations (buy+sell).
	// For PumpSwap/PumpAPI you mentioned: 0.25% buy + 0.25% sell.
	PumpFeeBuyPct  float64 `validate:"gte=0"` // PUMP_FEE_BUY_PCT
	PumpFeeSellPct float64 `validate:"gte=0"` // PUMP_FEE_SELL_PCT

	TradeSize    float64 `validate:"gte=0"` // TRADE_SIZE SOL statis; 0 jika pakai TRADE_SIZE_PCT
	TradeSizePct float64 `validate:"gte=0"` // TRADE_SIZE_PCT — persentase saldo wallet (0=off, pakai TRADE_SIZE)
	// Plafon satu BUY (setelah hitung TRADE_SIZE / TRADE_SIZE_PCT). 0 = tanpa plafon (hati-hati wallet gemuk).
	MaxTradeSizeSOL  float64       `validate:"gte=0"`
	TimeoutMS        int           `validate:"gt=0"`
	RecoveryInterval time.Duration `validate:"gt=0"`
	RPCStub          bool
	PaperTrade       bool // PAPER_TRADE — flag uji dengan RPC nyata; HELIUS_API_KEY membangun URL Helius mainnet jika RPC_URL kosong

	GraceSeconds    int     `validate:"gte=0"`
	GraceSL         float64 `validate:"gte=0"` // GRACE_SL — stop loss selama grace period (%)
	GraceTP         float64 `validate:"gte=0"` // GRACE_TP — take profit selama grace period (%; 0 = pakai TP_PERCENT)
	GraceTrailDrop  float64 `validate:"gte=0"` // GRACE_TRAIL_DROP — trailing drop dari peak di grace period (%; 0 = off, langsung TP)
	MinHold         int     `validate:"gt=0"`
	MaxHold         int     `validate:"gt=0"`
	TakeProfit      float64 `validate:"gte=0"`
	StopLoss        float64 `validate:"gte=0"`
	PanicSL         float64 `validate:"gte=0"`
	MomentumDrop    float64 `validate:"gte=0"`
	QuoteIntervalMS int     `validate:"gt=0"`
	ConfirmSLMS     int     `validate:"gte=0"` // CONFIRM_SL_MS — jeda konfirmasi SL/panic (ms), 0=langsung eksekusi
	// Event-driven exits (WSS).
	WhaleSellMinSOL float64 `validate:"gte=0"` // WHALE_SELL_MIN_SOL (0 = off)

	// PR-003 reporting (optional Telegram)
	TelegramBotToken   string `validate:"omitempty"`
	TelegramChatID     string `validate:"omitempty"`
	ReportEveryNTrades int    `validate:"gte=0"`
	ReportIntervalMin  int    `validate:"gte=0"`
	ReportMaxTrades    int    `validate:"gte=0"`

	// PR-005 profit guard (BUY gate; 0 = nonaktif untuk limit tertentu)
	MaxDailyLoss   float64 `validate:"gte=0"`
	MinBalance     float64 `validate:"gte=0"`
	EnableTrading  bool
	MaxDailyTrades int `validate:"gte=0"`
	// Positions terbuka bersamaan (setelah BUY sampai monitor selesai). 0 = tidak dibatasi.
	MaxOpenPositions int `validate:"gte=0"`

	// Production hazards (rlangga-env-contract.md §7)
	MinDust            float64 `validate:"gte=0"` // 0 = abaikan filter debu
	QuoteMaxAgeMS      int     `validate:"gte=0"` // 0 = tidak lewati tick monitor
	LockTTLMin         int     `validate:"gte=0"` // 0 pakai default paket lock (12 m)
	StaleBalanceWaitMS int     `validate:"gte=0"` // jeda pasca-SELL recovery (0 = off)

	// Pump WebSocket (PumpPortal wss://pumpportal.fun/api/data atau PumpApi wss://stream.pumpapi.io)
	PumpWSURL                   string `validate:"omitempty,url"`
	PumpWSSubscribeJSON         string `validate:"omitempty"`     // JSON array subscribe PumpPortal; kosong = default jika host pumpportal
	PumpWSFallbackURL           string `validate:"omitempty,url"` // stream sekunder paralel (dedupe mint di idempotency)
	PumpWSFallbackSubscribeJSON string `validate:"omitempty"`     // kosong = pakai PumpWSSubscribeJSON + default host
	PumpWSAutoHandle            bool   // true = parse mint dari pesan lalu HandleMint (hati-hati di produksi)
	SimulateTrading             bool   // SIMULATE_TRADING — stream: hanya log mint, tanpa BUY/monitor (kecuali SimulateEngine)
	SimulateEngine              bool   // SIMULATE_ENGINE — jalankan guard+lock+monitor+exit; BUY/SELL tanpa tx (quote nyata atau sintetis)
	// SIMULATE_USE_LIVE_BALANCE — dengan SIMULATE_ENGINE: saldo untuk guard/TRADE_SIZE_PCT dari RPC (bukan 5 SOL virtual).
	SimulateUseLiveBalance bool
	// Quote sintetis (hanya jika SIMULATE_ENGINE + quote HTTP kosong): amplitudo/osilasi PnL paper.
	SimulateSynthAmplitudePct float64 // SIMULATE_SYNTH_AMPLITUDE_PCT — 0 = default 12
	SimulateSynthPeriodSec    float64 // SIMULATE_SYNTH_PERIOD_SEC — 0 = default 4 (pembagi waktu di sin)
	SimulateSynthDriftPct     float64 // SIMULATE_SYNTH_DRIFT_PCT — bias % ditambah ke osilasi (bisa negatif)

	// Governance jam aktif BUY saja (rlangga-env-contract §6). -1 = nonaktif (24/7 untuk gate jam).
	TZ              string `validate:"omitempty"`
	ActiveStartHour int    `validate:"gte=-1,lte=23"`
	ActiveEndHour   int    `validate:"gte=-1,lte=23"`

	// Filter anti-rug / honeypot (RPC on-chain sebelum BUY; lihat docs/filter-rug-honeypot.md)
	FilterAntiRug               bool
	FilterRejectFreezeAuthority bool    // freeze authority pada mint — sering dipakai untuk kunci jual
	FilterRejectMintAuthority   bool    // mint authority masih ada — risiko inflasi tambahan supply
	FilterMaxTopHolderPct       float64 `validate:"gte=0,lte=100"` // 0 = mati; mis. 5 = tolak jika holder teratas > 5% supply
	FilterRPCFailOpen           bool    // true = jika RPC error, loloskan (jangan blok trading)
	// Wajibkan initialBuy di snapshot WSS (ParseStreamEvent); tanpa field ini → skip BUY.
	FilterRequireInitialBuy bool
	// Filter dari payload WebSocket (pra-RPC / pra-HandleMint); lihat docs/wss-data-for-filters.md
	FilterWSSAllowTxTypes []string // FILTER_WSS_ALLOW_TX_TYPES — daftar txType/event yang diizinkan (lowercase)
	FilterWSSDenyTxTypes  []string
	FilterWSSAllowMethods []string // FILTER_WSS_ALLOW_METHODS — method/channel yang diizinkan
	FilterWSSMinSOL       float64  // FILTER_WSS_MIN_SOL — SOL minimum dari payload (0 = off)
	FilterWSSMaxSOL       float64  // FILTER_WSS_MAX_SOL — SOL maksimum dari payload (0 = off)
	// FILTER_WSS_POOL — jika di-set, field pool dari payload harus cocok salah satu (lowercase); kosong = off.
	// Default yang disarankan di template: pump-amm saja (likuiditas AMM); tanpa bonding curve "pump" kecuali sengaja.
	FilterWSSPoolAllow       []string
	FilterWSSMinMarketCapSOL float64 // FILTER_WSS_MIN_MARKET_CAP_SOL — marketCapSol minimum (0 = off)
	FilterWSSMaxMarketCapSOL float64 // FILTER_WSS_MAX_MARKET_CAP_SOL — marketCapSol maksimum (0 = off)
	// WSS anti-scam (tanpa RPC): pool + token metadata dari payload.
	FilterWSSRequirePoolCreatedBy  []string // FILTER_WSS_REQUIRE_POOL_CREATED_BY — wajib poolCreatedBy termasuk salah satu (lowercase)
	FilterWSSMinBurnedLiquidityPct float64  `validate:"gte=0,lte=100"` // FILTER_WSS_MIN_BURNED_LIQUIDITY_PCT (WSS gate)
	FilterWSSMaxPoolFeeRate        float64  `validate:"gte=0"`         // FILTER_WSS_MAX_POOL_FEE_RATE (WSS gate)
	FilterWSSRejectMintAuthority   bool     // FILTER_WSS_REJECT_MINT_AUTHORITY (WSS gate)
	FilterWSSRejectFreezeAuthority bool     // FILTER_WSS_REJECT_FREEZE_AUTHORITY (WSS gate)
	FilterWSSDenyTokenExtensions   []string // FILTER_WSS_DENY_TOKEN_EXTENSIONS (lowercase)
	FilterWSSMinSolInPool          float64  `validate:"gte=0"` // FILTER_WSS_MIN_SOL_IN_POOL (WSS gate)
	// Smart entry filters (berbasis Mint Activity Tracker — sliding window event per mint).
	FilterMinBuySellRatio   float64 `validate:"gte=0"` // FILTER_MIN_BUY_SELL_RATIO — rasio buy/sell minimum sebelum entry (0=off)
	FilterMinTokenAgeSec    float64 `validate:"gte=0"` // FILTER_MIN_TOKEN_AGE_SEC — umur token minimum sejak pertama kali terlihat di stream (0=off)
	FilterRequireMcapRise   bool    // FILTER_REQUIRE_MCAP_RISE — wajib mcap naik (momentum positif) sebelum entry
	FilterActivityWindowSec float64 `validate:"gte=0"` // FILTER_ACTIVITY_WINDOW_SEC — lebar window untuk analisis aktivitas (default 30)
	// Analisis & filter snapshot entry (dari ParseStreamEvent; lihat docs/sql/trades.sql)
	TradeSQLitePath                 string  `validate:"omitempty"`     // TRADE_SQLITE_PATH — file .sqlite untuk query SQL (selain Redis)
	FilterMinInitialBuy             float64 `validate:"gte=0"`         // FILTER_MIN_INITIAL_BUY — tolak BUY jika initialBuy < ini (0 = off; perlu snapshot)
	FilterMinEntryMarketCapSOL      float64 `validate:"gte=0"`         // FILTER_MIN_ENTRY_MARKET_CAP_SOL — tolak jika marketCapSol < ini (0 = off)
	FilterMinEntrySolInPool         float64 `validate:"gte=0"`         // FILTER_MIN_ENTRY_SOL_IN_POOL — tolak jika solInPool < ini (0=off; butuh payload AMM)
	FilterMinBurnedLiquidityPct     float64 `validate:"gte=0,lte=100"` // FILTER_MIN_BURNED_LIQUIDITY_PCT — tolak jika burnedLiquidity < ini (0=off)
	FilterRejectPoolCreatedByCustom bool    // FILTER_REJECT_POOL_CREATED_BY_CUSTOM — tolak jika poolCreatedBy=custom

	MintCooldownSec int    `validate:"gte=0"`     // MINT_COOLDOWN_SEC — cooldown setelah trade per mint (default 300s / 5min)
	HealthPort      string `validate:"omitempty"` // HEALTH_PORT — port untuk HTTP health check (default 8080)
}

// FilterWSSGateActive true jika setidaknya satu gate WSS di env di-set (selain nol/kosong).
func (c *Config) FilterWSSGateActive() bool {
	if c == nil {
		return false
	}
	return len(c.FilterWSSAllowTxTypes) > 0 || len(c.FilterWSSDenyTxTypes) > 0 ||
		len(c.FilterWSSAllowMethods) > 0 || c.FilterWSSMinSOL > 0 || c.FilterWSSMaxSOL > 0 ||
		len(c.FilterWSSPoolAllow) > 0 || c.FilterWSSMinMarketCapSOL > 0 || c.FilterWSSMaxMarketCapSOL > 0 ||
		len(c.FilterWSSRequirePoolCreatedBy) > 0 || c.FilterWSSMinBurnedLiquidityPct > 0 || c.FilterWSSMaxPoolFeeRate > 0 ||
		c.FilterWSSRejectMintAuthority || c.FilterWSSRejectFreezeAuthority || len(c.FilterWSSDenyTokenExtensions) > 0 ||
		c.FilterWSSMinSolInPool > 0
}

// Load reads configuration from the environment, validates struct tags, then applies runtime guards.
// Sets global C on success. Safe to call once at startup (worker); tests may call repeatedly with t.Setenv.
func Load() (*Config, error) {
	c, err := parseEnv()
	if err != nil {
		return nil, err
	}
	if err := validate.Struct(c); err != nil {
		return nil, fmt.Errorf("config validate: %w", err)
	}
	if err := validateRuntimeGuards(c); err != nil {
		return nil, err
	}
	C = c
	return c, nil
}

func parseEnv() (*Config, error) {
	timeout, err := intFromEnv("TIMEOUT_MS", 1500, 1)
	if err != nil {
		return nil, err
	}
	recSec, err := intFromEnv("RECOVERY_INTERVAL", 10, 1)
	if err != nil {
		return nil, err
	}
	tradeSizePct, err := floatFromEnv("TRADE_SIZE_PCT", 0, 0, true)
	if err != nil {
		return nil, err
	}
	tradeDefault := 0.1
	allowZeroTrade := tradeSizePct > 0
	trade, err := floatFromEnv("TRADE_SIZE", tradeDefault, 0, allowZeroTrade)
	if err != nil {
		return nil, err
	}
	maxTradeSizeSOL, err := floatFromEnv("MAX_TRADE_SIZE_SOL", 0, 0, true)
	if err != nil {
		return nil, err
	}

	grace, err := intFromEnv("GRACE_SECONDS", 2, 0)
	if err != nil {
		return nil, err
	}
	graceSL, err := floatFromEnv("GRACE_SL", 0, 0, true)
	if err != nil {
		return nil, err
	}
	graceTP, err := floatFromEnv("GRACE_TP", 0, 0, true)
	if err != nil {
		return nil, err
	}
	graceTrailDrop, err := floatFromEnv("GRACE_TRAIL_DROP", 0, 0, true)
	if err != nil {
		return nil, err
	}
	mintCooldown, err := intFromEnv("MINT_COOLDOWN_SEC", 300, 0)
	if err != nil {
		return nil, err
	}
	minHold, err := intFromEnv("MIN_HOLD", 5, 1)
	if err != nil {
		return nil, err
	}
	maxHold, err := intFromEnv("MAX_HOLD", 15, 1)
	if err != nil {
		return nil, err
	}
	tp, err := floatFromEnv("TP_PERCENT", 7, 0, true)
	if err != nil {
		return nil, err
	}
	sl, err := floatFromEnv("SL_PERCENT", 5, 0, true)
	if err != nil {
		return nil, err
	}
	panicSL, err := floatFromEnv("PANIC_SL", 8, 0, true)
	if err != nil {
		return nil, err
	}
	mom, err := floatFromEnv("MOMENTUM_DROP", 2.5, 0, true)
	if err != nil {
		return nil, err
	}
	qms, err := intFromEnv("QUOTE_INTERVAL_MS", 500, 1)
	if err != nil {
		return nil, err
	}
	confirmSLMS, err := intFromEnv("CONFIRM_SL_MS", 0, 0)
	if err != nil {
		return nil, err
	}
	whaleSellMinSOL, err := floatFromEnv("WHALE_SELL_MIN_SOL", 0, 0, true)
	if err != nil {
		return nil, err
	}
	reportN, err := intFromEnv("REPORT_EVERY_N_TRADES", 5, 0)
	if err != nil {
		return nil, err
	}
	reportMin, err := intFromEnv("REPORT_INTERVAL_MIN", 30, 0)
	if err != nil {
		return nil, err
	}
	reportMax, err := intFromEnv("REPORT_LOAD_RECENT", 0, 0)
	if err != nil {
		return nil, err
	}

	maxDailyLoss, err := floatFromEnv("MAX_DAILY_LOSS", 0, 0, true)
	if err != nil {
		return nil, err
	}
	minBal, err := floatFromEnv("MIN_BALANCE", 0, 0, true)
	if err != nil {
		return nil, err
	}
	maxTrades, err := intFromEnv("MAX_DAILY_TRADES", 0, 0)
	if err != nil {
		return nil, err
	}
	maxOpenPos, err := intFromEnv("MAX_OPEN_POSITIONS", 0, 0)
	if err != nil {
		return nil, err
	}

	minDust, err := floatFromEnv("MIN_DUST", 0, 0, true)
	if err != nil {
		return nil, err
	}
	quoteMaxAgeMS, err := intFromEnv("QUOTE_MAX_AGE_MS", 0, 0)
	if err != nil {
		return nil, err
	}
	lockTTLMin, err := intFromEnv("LOCK_TTL_MIN", 12, 1)
	if err != nil {
		return nil, err
	}
	staleBalMS, err := intFromEnv("STALE_BALANCE_WAIT_MS", 0, 0)
	if err != nil {
		return nil, err
	}

	synthAmp, err := floatFromEnv("SIMULATE_SYNTH_AMPLITUDE_PCT", 0, 0, true)
	if err != nil {
		return nil, err
	}
	synthPeriod, err := floatFromEnv("SIMULATE_SYNTH_PERIOD_SEC", 0, 0, true)
	if err != nil {
		return nil, err
	}
	synthDrift, err := parseOptionalFloat("SIMULATE_SYNTH_DRIFT_PCT")
	if err != nil {
		return nil, err
	}

	tz := strings.TrimSpace(os.Getenv("TZ"))
	activeStart, activeEnd, err := parseActiveHours()
	if err != nil {
		return nil, err
	}

	filterAntiRug := parseBoolish(os.Getenv("FILTER_ANTI_RUG"), false)
	filterRejectFreeze := parseBoolish(os.Getenv("FILTER_REJECT_FREEZE_AUTHORITY"), true)
	filterRejectMint := parseBoolish(os.Getenv("FILTER_REJECT_MINT_AUTHORITY"), false)
	// Default 0: jangan tambah round-trip RPC (largest accounts) — sniping time-sensitive; set mis. 5 jika mau tolak konsentrasi tinggi.
	filterMaxTopPct, err := floatFromEnv("FILTER_MAX_TOP_HOLDER_PCT", 0, 0, true)
	if err != nil {
		return nil, err
	}
	if filterMaxTopPct > 100 {
		return nil, fmt.Errorf("FILTER_MAX_TOP_HOLDER_PCT: must be <= 100")
	}
	filterFailOpen := parseBoolish(os.Getenv("FILTER_RPC_FAIL_OPEN"), true)
	filterRequireInitialBuy := parseBoolish(os.Getenv("FILTER_REQUIRE_INITIAL_BUY"), false)

	filterWSSAllowTx := splitCommaLower(os.Getenv("FILTER_WSS_ALLOW_TX_TYPES"))
	filterWSSDenyTx := splitCommaLower(os.Getenv("FILTER_WSS_DENY_TX_TYPES"))
	filterWSSAllowMeth := splitCommaLower(os.Getenv("FILTER_WSS_ALLOW_METHODS"))
	filterWSSMinSOL, err := floatFromEnv("FILTER_WSS_MIN_SOL", 0, 0, true)
	if err != nil {
		return nil, err
	}
	filterWSSMaxSOL, err := floatFromEnv("FILTER_WSS_MAX_SOL", 0, 0, true)
	if err != nil {
		return nil, err
	}
	filterWSSPoolAllow := splitCommaLower(os.Getenv("FILTER_WSS_POOL"))
	filterWSSMinMcap, err := floatFromEnv("FILTER_WSS_MIN_MARKET_CAP_SOL", 0, 0, true)
	if err != nil {
		return nil, err
	}
	filterWSSMaxMcap, err := floatFromEnv("FILTER_WSS_MAX_MARKET_CAP_SOL", 0, 0, true)
	if err != nil {
		return nil, err
	}
	filterWSSRequireCreatedBy := splitCommaLower(os.Getenv("FILTER_WSS_REQUIRE_POOL_CREATED_BY"))
	filterWSSMinBurnPct, err := floatFromEnv("FILTER_WSS_MIN_BURNED_LIQUIDITY_PCT", 0, 0, true)
	if err != nil {
		return nil, err
	}
	filterWSSMaxFeeRate, err := floatFromEnv("FILTER_WSS_MAX_POOL_FEE_RATE", 0, 0, true)
	if err != nil {
		return nil, err
	}
	filterWSSRejectMintAuth := parseBoolish(os.Getenv("FILTER_WSS_REJECT_MINT_AUTHORITY"), false)
	filterWSSRejectFreezeAuth := parseBoolish(os.Getenv("FILTER_WSS_REJECT_FREEZE_AUTHORITY"), false)
	filterWSSDenyExt := splitCommaLower(os.Getenv("FILTER_WSS_DENY_TOKEN_EXTENSIONS"))
	tradeSQLitePath := strings.TrimSpace(os.Getenv("TRADE_SQLITE_PATH"))
	filterMinIB, err := floatFromEnv("FILTER_MIN_INITIAL_BUY", 0, 0, true)
	if err != nil {
		return nil, err
	}
	filterMinEntryMcap, err := floatFromEnv("FILTER_MIN_ENTRY_MARKET_CAP_SOL", 0, 0, true)
	if err != nil {
		return nil, err
	}
	filterMinEntrySolInPool, err := floatFromEnv("FILTER_MIN_ENTRY_SOL_IN_POOL", 0, 0, true)
	if err != nil {
		return nil, err
	}
	filterMinBurnPct, err := floatFromEnv("FILTER_MIN_BURNED_LIQUIDITY_PCT", 0, 0, true)
	if err != nil {
		return nil, err
	}
	filterRejectCustom := parseBoolish(os.Getenv("FILTER_REJECT_POOL_CREATED_BY_CUSTOM"), false)

	filterWSSMinSolInPool, err := floatFromEnv("FILTER_WSS_MIN_SOL_IN_POOL", 0, 0, true)
	if err != nil {
		return nil, err
	}
	filterMinBuySellRatio, err := floatFromEnv("FILTER_MIN_BUY_SELL_RATIO", 0, 0, true)
	if err != nil {
		return nil, err
	}
	filterMinTokenAgeSec, err := floatFromEnv("FILTER_MIN_TOKEN_AGE_SEC", 0, 0, true)
	if err != nil {
		return nil, err
	}
	filterRequireMcapRise := parseBoolish(os.Getenv("FILTER_REQUIRE_MCAP_RISE"), false)
	filterActivityWindowSec, err := floatFromEnv("FILTER_ACTIVITY_WINDOW_SEC", 30, 0, true)
	if err != nil {
		return nil, err
	}

	pumpSlippage, err := floatFromEnv("PUMP_SLIPPAGE", 10, 0, true)
	if err != nil {
		return nil, err
	}
	pumpAPISlippage, err := floatFromEnv("PUMPAPI_SLIPPAGE", 20, 0, true)
	if err != nil {
		return nil, err
	}
	pumpPriorityFee, err := floatFromEnv("PUMP_PRIORITY_FEE", 0.00005, 0, true)
	if err != nil {
		return nil, err
	}
	feeBuyPct, err := floatFromEnv("PUMP_FEE_BUY_PCT", 0.25, 0, true)
	if err != nil {
		return nil, err
	}
	feeSellPct, err := floatFromEnv("PUMP_FEE_SELL_PCT", 0.25, 0, true)
	if err != nil {
		return nil, err
	}
	pumpNative := parseBoolish(os.Getenv("PUMP_NATIVE"), false)
	pumpPortalAPIKey := strings.TrimSpace(os.Getenv("PUMPPORTAL_API_KEY"))
	pumpPortalQuoteURL := strings.TrimSpace(os.Getenv("PUMPPORTAL_QUOTE_URL"))
	pumpAPIQuoteURL := strings.TrimSpace(os.Getenv("PUMPAPI_QUOTE_URL"))
	walletPub := strings.TrimSpace(os.Getenv("PUMP_WALLET_PUBLIC_KEY"))
	walletPriv := strings.TrimSpace(os.Getenv("PUMP_PRIVATE_KEY"))
	pumpAPIQuoteMint := strings.TrimSpace(os.Getenv("PUMPAPI_QUOTE_MINT"))
	pumpAPIPoolID := strings.TrimSpace(os.Getenv("PUMPAPI_POOL_ID"))
	pumpAPIGuaranteed := parseBoolish(os.Getenv("PUMPAPI_GUARANTEED_DELIVERY"), false)
	pumpAPIJitoTip, err := floatFromEnv("PUMPAPI_JITO_TIP", 0, 0, true)
	if err != nil {
		return nil, err
	}
	pumpAPIMaxPri, err := floatFromEnv("PUMPAPI_MAX_PRIORITY_FEE", 0, 0, true)
	if err != nil {
		return nil, err
	}
	pumpAPIPriMode := strings.TrimSpace(os.Getenv("PUMPAPI_PRIORITY_FEE_MODE"))

	stub := os.Getenv("RPC_STUB") == "1" || os.Getenv("RPC_STUB") == "true"

	paperTrade := parseBoolish(os.Getenv("PAPER_TRADE"), false)
	rpcURL := strings.TrimSpace(os.Getenv("RPC_URL"))
	heliusKey := strings.TrimSpace(os.Getenv("HELIUS_API_KEY"))
	if paperTrade && rpcURL == "" && heliusKey != "" {
		rpcURL = "https://mainnet.helius-rpc.com/?api-key=" + url.QueryEscape(heliusKey)
	}
	if paperTrade && rpcURL == "" {
		return nil, fmt.Errorf("PAPER_TRADE=1 requires RPC_URL or HELIUS_API_KEY")
	}

	rpcURLs, err := parseRPCURLsFromEnv(rpcURL)
	if err != nil {
		return nil, err
	}
	rpcPrimary := rpcURL
	if len(rpcURLs) > 0 {
		rpcPrimary = rpcURLs[0]
	}

	pumpWSURL := strings.TrimSpace(os.Getenv("PUMP_WS_URL"))
	pumpWSFallbackURL := strings.TrimSpace(os.Getenv("PUMP_WS_FALLBACK_URL"))

	c := &Config{
		RedisURL:                        os.Getenv("REDIS_URL"),
		RPCURL:                          rpcPrimary,
		RPCURLs:                         rpcURLs,
		PumpPortalURL:                   os.Getenv("PUMPPORTAL_URL"),
		PumpAPIURL:                      os.Getenv("PUMPAPI_URL"),
		PumpNative:                      pumpNative,
		PumpPortalAPIKey:                pumpPortalAPIKey,
		PumpPortalQuoteURL:              pumpPortalQuoteURL,
		PumpAPIQuoteURL:                 pumpAPIQuoteURL,
		WalletPublicKey:                 walletPub,
		PumpPrivateKey:                  walletPriv,
		PumpAPIQuoteMint:                pumpAPIQuoteMint,
		PumpAPIPoolID:                   pumpAPIPoolID,
		PumpAPIGuaranteedDelivery:       pumpAPIGuaranteed,
		PumpAPIJitoTip:                  pumpAPIJitoTip,
		PumpAPIMaxPriorityFee:           pumpAPIMaxPri,
		PumpAPIPriorityFeeMode:          pumpAPIPriMode,
		PumpSlippage:                    pumpSlippage,
		PumpAPISlippage:                 pumpAPISlippage,
		PumpPriorityFee:                 pumpPriorityFee,
		PumpFeeBuyPct:                   feeBuyPct,
		PumpFeeSellPct:                  feeSellPct,
		TradeSize:                       trade,
		TradeSizePct:                    tradeSizePct,
		MaxTradeSizeSOL:                 maxTradeSizeSOL,
		TimeoutMS:                       timeout,
		RecoveryInterval:                time.Duration(recSec) * time.Second,
		RPCStub:                         stub,
		PaperTrade:                      paperTrade,
		GraceSeconds:                    grace,
		GraceSL:                         graceSL,
		GraceTP:                         graceTP,
		GraceTrailDrop:                  graceTrailDrop,
		MinHold:                         minHold,
		MaxHold:                         maxHold,
		TakeProfit:                      tp,
		StopLoss:                        sl,
		PanicSL:                         panicSL,
		MomentumDrop:                    mom,
		QuoteIntervalMS:                 qms,
		ConfirmSLMS:                     confirmSLMS,
		WhaleSellMinSOL:                 whaleSellMinSOL,
		TelegramBotToken:                os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:                  os.Getenv("TELEGRAM_CHAT_ID"),
		ReportEveryNTrades:              reportN,
		ReportIntervalMin:               reportMin,
		ReportMaxTrades:                 reportMax,
		MaxDailyLoss:                    maxDailyLoss,
		MinBalance:                      minBal,
		EnableTrading:                   parseEnableTrading(true),
		MaxDailyTrades:                  maxTrades,
		MaxOpenPositions:                maxOpenPos,
		MinDust:                         minDust,
		QuoteMaxAgeMS:                   quoteMaxAgeMS,
		LockTTLMin:                      lockTTLMin,
		StaleBalanceWaitMS:              staleBalMS,
		PumpWSURL:                       pumpWSURL,
		PumpWSSubscribeJSON:             strings.TrimSpace(os.Getenv("PUMP_WS_SUBSCRIBE_JSON")),
		PumpWSFallbackURL:               pumpWSFallbackURL,
		PumpWSFallbackSubscribeJSON:     strings.TrimSpace(os.Getenv("PUMP_WS_FALLBACK_SUBSCRIBE_JSON")),
		PumpWSAutoHandle:                parseBoolish(os.Getenv("PUMP_WS_AUTO_HANDLE"), false),
		SimulateTrading:                 parseBoolish(os.Getenv("SIMULATE_TRADING"), false),
		SimulateEngine:                  parseBoolish(os.Getenv("SIMULATE_ENGINE"), false),
		SimulateUseLiveBalance:          parseBoolish(os.Getenv("SIMULATE_USE_LIVE_BALANCE"), false),
		SimulateSynthAmplitudePct:       synthAmp,
		SimulateSynthPeriodSec:          synthPeriod,
		SimulateSynthDriftPct:           synthDrift,
		TZ:                              tz,
		ActiveStartHour:                 activeStart,
		ActiveEndHour:                   activeEnd,
		FilterAntiRug:                   filterAntiRug,
		FilterRejectFreezeAuthority:     filterRejectFreeze,
		FilterRejectMintAuthority:       filterRejectMint,
		FilterMaxTopHolderPct:           filterMaxTopPct,
		FilterRPCFailOpen:               filterFailOpen,
		FilterRequireInitialBuy:         filterRequireInitialBuy,
		FilterWSSAllowTxTypes:           filterWSSAllowTx,
		FilterWSSDenyTxTypes:            filterWSSDenyTx,
		FilterWSSAllowMethods:           filterWSSAllowMeth,
		FilterWSSMinSOL:                 filterWSSMinSOL,
		FilterWSSMaxSOL:                 filterWSSMaxSOL,
		FilterWSSPoolAllow:              filterWSSPoolAllow,
		FilterWSSMinMarketCapSOL:        filterWSSMinMcap,
		FilterWSSMaxMarketCapSOL:        filterWSSMaxMcap,
		FilterWSSRequirePoolCreatedBy:   filterWSSRequireCreatedBy,
		FilterWSSMinBurnedLiquidityPct:  filterWSSMinBurnPct,
		FilterWSSMaxPoolFeeRate:         filterWSSMaxFeeRate,
		FilterWSSRejectMintAuthority:    filterWSSRejectMintAuth,
		FilterWSSRejectFreezeAuthority:  filterWSSRejectFreezeAuth,
		FilterWSSDenyTokenExtensions:    filterWSSDenyExt,
		FilterWSSMinSolInPool:           filterWSSMinSolInPool,
		FilterMinBuySellRatio:           filterMinBuySellRatio,
		FilterMinTokenAgeSec:            filterMinTokenAgeSec,
		FilterRequireMcapRise:           filterRequireMcapRise,
		FilterActivityWindowSec:         filterActivityWindowSec,
		TradeSQLitePath:                 tradeSQLitePath,
		FilterMinInitialBuy:             filterMinIB,
		FilterMinEntryMarketCapSOL:      filterMinEntryMcap,
		FilterMinEntrySolInPool:         filterMinEntrySolInPool,
		FilterMinBurnedLiquidityPct:     filterMinBurnPct,
		FilterRejectPoolCreatedByCustom: filterRejectCustom,
		MintCooldownSec:                 mintCooldown,
		HealthPort:                      strings.TrimSpace(os.Getenv("HEALTH_PORT")),
	}
	return c, nil
}

func splitCommaLower(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		t := strings.ToLower(strings.TrimSpace(p))
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// parseActiveHours reads ACTIVE_START_HOUR / ACTIVE_END_HOUR. Both empty → -1,-1 (disabled).
// If only one is set, returns an error.
func parseActiveHours() (start, end int, err error) {
	start, end = -1, -1
	ash := strings.TrimSpace(os.Getenv("ACTIVE_START_HOUR"))
	aeh := strings.TrimSpace(os.Getenv("ACTIVE_END_HOUR"))
	if ash == "" && aeh == "" {
		return start, end, nil
	}
	if ash == "" || aeh == "" {
		return 0, 0, fmt.Errorf("ACTIVE_START_HOUR and ACTIVE_END_HOUR must both be set or both empty")
	}
	si, err := strconv.Atoi(ash)
	if err != nil || si < 0 || si > 23 {
		return 0, 0, fmt.Errorf("ACTIVE_START_HOUR: must be integer 0-23")
	}
	ei, err := strconv.Atoi(aeh)
	if err != nil || ei < 0 || ei > 23 {
		return 0, 0, fmt.Errorf("ACTIVE_END_HOUR: must be integer 0-23")
	}
	return si, ei, nil
}

// parseRPCURLsFromEnv: RPC_URLS=koma (prioritas) atau satu URL dari argumen fallback (RPC_URL).
func parseRPCURLsFromEnv(fallbackSingle string) ([]string, error) {
	raw := strings.TrimSpace(os.Getenv("RPC_URLS"))
	if raw != "" {
		var out []string
		for _, part := range strings.Split(raw, ",") {
			p := strings.TrimSpace(part)
			if p == "" {
				continue
			}
			u, err := url.Parse(p)
			if err != nil || u.Scheme == "" || u.Host == "" {
				return nil, fmt.Errorf("RPC_URLS: invalid URL %q", p)
			}
			out = append(out, p)
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("RPC_URLS: empty after parsing")
		}
		return out, nil
	}
	fb := strings.TrimSpace(fallbackSingle)
	if fb != "" {
		return []string{fb}, nil
	}
	return nil, nil
}

func parseBoolish(raw string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(raw))
	if v == "" {
		return def
	}
	if v == "0" || v == "false" || v == "no" || v == "off" {
		return false
	}
	if v == "1" || v == "true" || v == "yes" || v == "on" {
		return true
	}
	return def
}

func parseEnableTrading(def bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("ENABLE_TRADING")))
	if v == "" {
		return def
	}
	if v == "0" || v == "false" || v == "no" || v == "off" {
		return false
	}
	if v == "1" || v == "true" || v == "yes" || v == "on" {
		return true
	}
	return def
}

// intFromEnv parses int; empty → default; min is minimum allowed (inclusive).
func intFromEnv(key string, def, min int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	if n < min {
		return 0, fmt.Errorf("%s: must be >= %d", key, min)
	}
	return n, nil
}

// floatFromEnv parses float64; empty → default. allowZero uses min 0 for non-empty values.
// parseOptionalFloat membaca float dari env; kosong → 0. Nilai negatif diperbolehkan (untuk SIMULATE_SYNTH_DRIFT_PCT).
func parseOptionalFloat(key string) (float64, error) {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return f, nil
}

func floatFromEnv(key string, def float64, min float64, allowZero bool) (float64, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	if allowZero {
		if f < min {
			return 0, fmt.Errorf("%s: must be >= %v", key, min)
		}
	} else if f <= min {
		return 0, fmt.Errorf("%s: must be > %v", key, min)
	}
	return f, nil
}

// validateRuntimeGuards encodes cross-field and operational rules (validator tags are per-field only).
func validateRuntimeGuards(c *Config) error {
	if c.TradeSize <= 0 && c.TradeSizePct <= 0 {
		return fmt.Errorf("config runtime: TRADE_SIZE or TRADE_SIZE_PCT must be > 0")
	}
	if c.MinHold > c.MaxHold {
		return fmt.Errorf("config runtime: MIN_HOLD (%d) must be <= MAX_HOLD (%d)", c.MinHold, c.MaxHold)
	}
	if c.PanicSL <= 0 {
		return fmt.Errorf("config runtime: PANIC_SL must be > 0 (got %v); 0 would trigger panic exit on any non-positive PnL", c.PanicSL)
	}
	if c.StopLoss <= 0 {
		return fmt.Errorf("config runtime: SL_PERCENT must be > 0 (got %v); 0 would trigger stop-loss on any non-positive PnL", c.StopLoss)
	}
	if c.MomentumDrop <= 0 {
		return fmt.Errorf("config runtime: MOMENTUM_DROP must be > 0 (got %v); 0 would trigger momentum exit on every tick", c.MomentumDrop)
	}
	if !c.RPCStub && len(c.RPCURLs) == 0 && strings.TrimSpace(c.RPCURL) == "" {
		return fmt.Errorf("config runtime: RPC_URL or RPC_URLS is required when RPC_STUB is off")
	}
	if c.PaperTrade && c.RPCStub {
		return fmt.Errorf("config runtime: PAPER_TRADE requires real RPC calls; set RPC_STUB=0 (RPC_STUB=1 skips RPC)")
	}
	if (c.ActiveStartHour < 0) != (c.ActiveEndHour < 0) {
		return fmt.Errorf("config runtime: ACTIVE_START_HOUR and ACTIVE_END_HOUR must both be unset or both 0-23")
	}
	if c.FilterWSSMinSOL > 0 && c.FilterWSSMaxSOL > 0 && c.FilterWSSMinSOL > c.FilterWSSMaxSOL {
		return fmt.Errorf("config runtime: FILTER_WSS_MIN_SOL must be <= FILTER_WSS_MAX_SOL when both are set")
	}
	if c.FilterWSSMinMarketCapSOL > 0 && c.FilterWSSMaxMarketCapSOL > 0 && c.FilterWSSMinMarketCapSOL > c.FilterWSSMaxMarketCapSOL {
		return fmt.Errorf("config runtime: FILTER_WSS_MIN_MARKET_CAP_SOL must be <= FILTER_WSS_MAX_MARKET_CAP_SOL when both are set")
	}
	if !c.RPCStub && c.EnableTrading && strings.TrimSpace(c.WalletPublicKey) == "" {
		return fmt.Errorf("config runtime: PUMP_WALLET_PUBLIC_KEY is required when RPC_STUB=0 and ENABLE_TRADING=true (real SOL balance via getBalance)")
	}
	return nil
}
