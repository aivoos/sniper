package config

import "testing"

func TestIsUnsetPumpURL(t *testing.T) {
	if !IsUnsetPumpURL("") || !IsUnsetPumpURL("  ") || !IsUnsetPumpURL("xxx") || !IsUnsetPumpURL("XXX") {
		t.Fatal("expected unset")
	}
	if IsUnsetPumpURL("https://api.pumpapi.io") {
		t.Fatal("expected set")
	}
}
