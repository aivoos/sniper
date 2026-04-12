package redisx

import "testing"

func TestInit_Invalid(t *testing.T) {
	if err := Init(""); err == nil {
		t.Fatal("empty addr should error")
	}
}
