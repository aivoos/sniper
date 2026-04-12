package pnl

// CalcPnL returns percent PnL from buy vs simulate-sell quote (SOL).
func CalcPnL(buySOL, quoteSOL float64) float64 {
	if buySOL <= 0 {
		return 0
	}
	return (quoteSOL - buySOL) / buySOL * 100
}
