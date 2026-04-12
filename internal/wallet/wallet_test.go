package wallet

import "testing"

func TestGetSOLBalance(t *testing.T) {
	if GetSOLBalance() != 1.0 {
		t.Fatalf("stub balance: %v", GetSOLBalance())
	}
}
