package pumpws

import "testing"

func TestExtractMint_TopLevel(t *testing.T) {
	msg := []byte(`{"mint":"So11111111111111111111111111111111111111112","x":1}`)
	if got := ExtractMint(msg); got != "So11111111111111111111111111111111111111112" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractMint_Nested(t *testing.T) {
	msg := []byte(`{"data":{"token":"So11111111111111111111111111111111111111112"}}`)
	if got := ExtractMint(msg); got != "So11111111111111111111111111111111111111112" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractMint_InvalidJSON(t *testing.T) {
	if ExtractMint([]byte(`not json`)) != "" {
		t.Fatal("expected empty")
	}
}

func TestLooksLikeMint_TooShort(t *testing.T) {
	if looksLikeMint("short") {
		t.Fatal("expected false")
	}
}
