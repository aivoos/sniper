package wallet

import "testing"

func TestGetSOLBalance_Default(t *testing.T) {
	BalanceHook = nil
	if GetSOLBalance() != 1.0 {
		t.Fatal(GetSOLBalance())
	}
}

func TestGetSOLBalance_Hook(t *testing.T) {
	BalanceHook = func() float64 { return 3.14 }
	t.Cleanup(func() { BalanceHook = nil })
	if GetSOLBalance() != 3.14 {
		t.Fatal(GetSOLBalance())
	}
}
