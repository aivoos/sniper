package wallet

// BalanceHook, if set, overrides GetSOLBalance (tests).
var BalanceHook func() float64

// GetSOLBalance returns SOL balance for the trading wallet (stub until RPC wallet integration).
func GetSOLBalance() float64 {
	if BalanceHook != nil {
		return BalanceHook()
	}
	return 1.0
}
