package pumpws

import (
	"bytes"
	"encoding/json"
)

// decodeJSONRoot mem-parsing JSON dengan UseNumber agar angka besar tidak mengambang.
func decodeJSONRoot(msg []byte, v *interface{}) error {
	dec := json.NewDecoder(bytes.NewReader(msg))
	dec.UseNumber()
	return dec.Decode(v)
}

// ExtractMint mencoba mengambil mint address dari payload JSON event (PumpPortal / PumpApi).
// Hanya mengembalikan string yang panjangnya masuk akal untuk pubkey Solana base58.
func ExtractMint(msg []byte) string {
	var v interface{}
	if decodeJSONRoot(msg, &v) != nil {
		return ""
	}
	return walkForMint(v)
}

func walkForMint(v interface{}) string {
	switch t := v.(type) {
	case map[string]interface{}:
		for _, k := range []string{"mint", "tokenMint", "mint_address", "address", "ca", "token"} {
			if s, ok := t[k].(string); ok && looksLikeMint(s) {
				return s
			}
		}
		for _, val := range t {
			if m := walkForMint(val); m != "" {
				return m
			}
		}
	case []interface{}:
		for _, el := range t {
			if m := walkForMint(el); m != "" {
				return m
			}
		}
	}
	return ""
}

func looksLikeMint(s string) bool {
	n := len(s)
	if n < 32 || n > 48 {
		return false
	}
	for _, c := range s {
		if (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			continue
		}
		return false
	}
	return true
}
