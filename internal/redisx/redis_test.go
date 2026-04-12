package redisx

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func TestInit_Invalid(t *testing.T) {
	if err := Init(""); err == nil {
		t.Fatal("empty addr should error")
	}
}

func TestInit_OK(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if Client != nil {
			_ = Client.Close()
		}
		Client = nil
		s.Close()
	})
	if err := Init(s.Addr()); err != nil {
		t.Fatal(err)
	}
	if Client == nil {
		t.Fatal("client should be set")
	}
}
