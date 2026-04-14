package safego_test

import (
	"sync"
	"testing"
	"time"

	"rlangga/internal/safego"
)

func TestGo_Runs(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	safego.Go("test", func() { wg.Done() })
	wg.Wait()
}

func TestGo_RecoversPanic(t *testing.T) {
	safego.Go("panic-test", func() { panic("intentional test panic") })
	time.Sleep(100 * time.Millisecond)
}
