package recovery

import "testing"

func TestRecoverAll_NoPanic(t *testing.T) {
	RecoverAll()
}
