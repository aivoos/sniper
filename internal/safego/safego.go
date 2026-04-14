package safego

import (
	"fmt"
	"runtime/debug"

	"rlangga/internal/log"
)

// Go runs fn in a new goroutine with panic recovery.
// If fn panics, the stack trace is logged and the goroutine exits cleanly.
func Go(label string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error(fmt.Sprintf("PANIC RECOVERED [%s]: %v\n%s", label, r, debug.Stack()))
			}
		}()
		fn()
	}()
}
