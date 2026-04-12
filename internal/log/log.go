package log

import (
	"fmt"
	"os"
)

// Info prints a line to stdout (structured logging can replace this later).
func Info(msg string) {
	fmt.Fprintln(os.Stdout, msg)
}

// Error prints a line to stderr.
func Error(msg string) {
	fmt.Fprintln(os.Stderr, msg)
}
