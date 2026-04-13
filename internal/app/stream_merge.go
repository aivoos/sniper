package app

import (
	"sync"

	"rlangga/internal/pumpws"
)

// streamMintGate menggabungkan dua koneksi WebSocket (primer + fallback) ke satu jalur kerja:
// event mint yang sama tidak menjalankan HandleMint bersamaan. Setelah HandleMint selesai,
// mint yang sama boleh diproses lagi (Redis idempotency / guard menentukan langkah berikutnya).
var streamMintGate sync.Map

func dispatchStreamMint(mint string) {
	dispatchStreamMintWithEntry(mint, nil)
}

func dispatchStreamMintEvent(ev pumpws.StreamEvent) {
	if ev.Mint == "" {
		return
	}
	dispatchStreamMintWithEntry(ev.Mint, &ev)
}

func dispatchStreamMintWithEntry(mint string, entry *pumpws.StreamEvent) {
	if mint == "" {
		return
	}
	if _, loaded := streamMintGate.LoadOrStore(mint, struct{}{}); loaded {
		return
	}
	go func() {
		defer streamMintGate.Delete(mint)
		HandleMint(mint, entry)
	}()
}
