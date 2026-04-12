package log

import "testing"

func TestInfo_Error(t *testing.T) {
	Info("ok")
	Error("err")
}
