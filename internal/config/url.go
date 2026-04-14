package config

import "strings"

// IsUnsetPumpURL true jika URL kosong atau placeholder lama ("xxx") dari template.
func IsUnsetPumpURL(s string) bool {
	s = strings.TrimSpace(s)
	return s == "" || strings.EqualFold(s, "xxx")
}
