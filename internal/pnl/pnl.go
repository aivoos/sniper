package pnl

// CalcPnL returns percent PnL from buy vs simulate-sell quote (SOL).
func CalcPnL(buySOL, quoteSOL float64) float64 {
	if buySOL <= 0 {
		return 0
	}
	return (quoteSOL - buySOL) / buySOL * 100
}

// ApplyFees returns fee-adjusted buy cost and sell proceeds.
// buyFeePct and sellFeePct are percent values (e.g. 0.25 means 0.25%).
func ApplyFees(buySOL, sellSOL, buyFeePct, sellFeePct float64) (buyCostSOL, sellProceedsSOL float64) {
	buyCostSOL = buySOL * (1 + buyFeePct/100)
	sellProceedsSOL = sellSOL * (1 - sellFeePct/100)
	return buyCostSOL, sellProceedsSOL
}
