package filter

import (
	"strings"

	"rlangga/internal/config"
	"rlangga/internal/pumpws"
)

// AllowStreamEvent menerapkan gate berbasis data WSS (pra-RPC) jika ada yang diaktifkan di config.
// Jika tidak ada gate WSS, selalu lolos.
func AllowStreamEvent(ev *pumpws.StreamEvent) (ok bool, reason string) {
	if ev == nil {
		return true, ""
	}
	cfg := config.C
	if cfg == nil || !cfg.FilterWSSGateActive() {
		return true, ""
	}

	if len(cfg.FilterWSSAllowTxTypes) > 0 {
		tx := strings.ToLower(strings.TrimSpace(ev.TxType))
		if tx == "" {
			return false, "tx type empty (FILTER_WSS_ALLOW_TX_TYPES active)"
		}
		if !inFoldList(tx, cfg.FilterWSSAllowTxTypes) {
			return false, "tx type not in allow list: " + ev.TxType
		}
	}

	if len(cfg.FilterWSSDenyTxTypes) > 0 {
		tx := strings.ToLower(strings.TrimSpace(ev.TxType))
		if tx != "" && inFoldList(tx, cfg.FilterWSSDenyTxTypes) {
			return false, "tx type denied: " + ev.TxType
		}
	}

	if len(cfg.FilterWSSAllowMethods) > 0 {
		m := strings.ToLower(strings.TrimSpace(ev.Method))
		if m == "" {
			return false, "method empty (FILTER_WSS_ALLOW_METHODS active)"
		}
		if !inFoldList(m, cfg.FilterWSSAllowMethods) {
			return false, "method not in allow list: " + ev.Method
		}
	}

	if cfg.FilterWSSMinSOL > 0 {
		if !ev.HasSolAmount {
			return false, "sol amount missing (FILTER_WSS_MIN_SOL active)"
		}
		if ev.SolAmount < cfg.FilterWSSMinSOL {
			return false, "sol below min"
		}
	}
	if cfg.FilterWSSMaxSOL > 0 {
		if !ev.HasSolAmount {
			return false, "sol amount missing (FILTER_WSS_MAX_SOL active)"
		}
		if ev.SolAmount > cfg.FilterWSSMaxSOL {
			return false, "sol above max"
		}
	}

	if len(cfg.FilterWSSPoolAllow) > 0 {
		p := strings.ToLower(strings.TrimSpace(ev.Pool))
		if p == "" {
			return false, "pool empty (FILTER_WSS_POOL active)"
		}
		if !inFoldList(p, cfg.FilterWSSPoolAllow) {
			return false, "pool not in allow list: " + ev.Pool
		}
	}
	if cfg.FilterWSSMinMarketCapSOL > 0 {
		if !ev.HasMarketCapSOL {
			return false, "market cap missing (FILTER_WSS_MIN_MARKET_CAP_SOL active)"
		}
		if ev.MarketCapSOL < cfg.FilterWSSMinMarketCapSOL {
			return false, "market cap below min"
		}
	}
	if cfg.FilterWSSMaxMarketCapSOL > 0 {
		if !ev.HasMarketCapSOL {
			return false, "market cap missing (FILTER_WSS_MAX_MARKET_CAP_SOL active)"
		}
		if ev.MarketCapSOL > cfg.FilterWSSMaxMarketCapSOL {
			return false, "market cap above max"
		}
	}

	if cfg.FilterWSSMinSolInPool > 0 {
		if !ev.HasSolInPool {
			return false, "solInPool missing (FILTER_WSS_MIN_SOL_IN_POOL active)"
		}
		if ev.SolInPool < cfg.FilterWSSMinSolInPool {
			return false, "solInPool below min"
		}
	}

	// Pool origin / rug checks (PumpAPI stream metadata).
	if len(cfg.FilterWSSRequirePoolCreatedBy) > 0 {
		by := strings.ToLower(strings.TrimSpace(ev.PoolCreatedBy))
		if by == "" {
			return false, "poolCreatedBy missing (FILTER_WSS_REQUIRE_POOL_CREATED_BY active)"
		}
		if !inFoldList(by, cfg.FilterWSSRequirePoolCreatedBy) {
			return false, "poolCreatedBy not in allow list: " + ev.PoolCreatedBy
		}
	}
	if cfg.FilterWSSMinBurnedLiquidityPct > 0 {
		if !ev.HasBurnedLiquidity {
			return false, "burnedLiquidity missing (FILTER_WSS_MIN_BURNED_LIQUIDITY_PCT active)"
		}
		if ev.BurnedLiquidityPct < cfg.FilterWSSMinBurnedLiquidityPct {
			return false, "burnedLiquidity below min"
		}
	}
	if cfg.FilterWSSMaxPoolFeeRate > 0 {
		if !ev.HasPoolFeeRate {
			return false, "poolFeeRate missing (FILTER_WSS_MAX_POOL_FEE_RATE active)"
		}
		if ev.PoolFeeRate > cfg.FilterWSSMaxPoolFeeRate {
			return false, "poolFeeRate above max"
		}
	}

	// Token technical scam checks (spl-token / spl-token-2022).
	if cfg.FilterWSSRejectMintAuthority && ev.HasMintAuthority {
		return false, "mintAuthority present (FILTER_WSS_REJECT_MINT_AUTHORITY active)"
	}
	if cfg.FilterWSSRejectFreezeAuthority && ev.HasFreezeAuthority {
		return false, "freezeAuthority present (FILTER_WSS_REJECT_FREEZE_AUTHORITY active)"
	}
	if len(cfg.FilterWSSDenyTokenExtensions) > 0 && len(ev.Extensions) > 0 {
		for _, ext := range ev.Extensions {
			if inFoldList(ext, cfg.FilterWSSDenyTokenExtensions) {
				return false, "token extension denied: " + ext
			}
		}
	}

	return true, ""
}

func inFoldList(val string, list []string) bool {
	for _, x := range list {
		if strings.EqualFold(strings.TrimSpace(x), val) {
			return true
		}
	}
	return false
}
