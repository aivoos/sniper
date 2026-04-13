package rpc

import (
	"encoding/json"
	"strings"

	"rlangga/internal/config"
)

// Program SPL Token (legacy) dan Token-2022 — keduanya dipindai untuk recovery.
const (
	splTokenProgram    = "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	token2022ProgramID = "TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb"
)

// GetSOLBalanceForPubkey returns SOL balance via getBalance (lamports / 1e9). ok=false jika RPC gagal atau pubkey kosong.
func GetSOLBalanceForPubkey(pubkey string) (sol float64, ok bool) {
	pk := strings.TrimSpace(pubkey)
	if pk == "" {
		return 0, false
	}
	var res struct {
		Value uint64 `json:"value"`
	}
	// commitment opsional — banyak node menerima hanya pubkey
	if err := callRPC("getBalance", []interface{}{pk}, &res); err != nil {
		return 0, false
	}
	return float64(res.Value) / 1e9, true
}

// GetWalletTokens lists SPL token accounts for the configured trading wallet (jsonParsed).
// Mengembalikan kosong jika RPC stub, tanpa pubkey, atau RPC gagal.
func getWalletTokensFromRPC() []Token {
	cfg := config.C
	if cfg == nil || cfg.RPCStub {
		return nil
	}
	pk := strings.TrimSpace(cfg.WalletPublicKey)
	if pk == "" {
		return nil
	}
	var out []Token
	for _, program := range []string{splTokenProgram, token2022ProgramID} {
		part := fetchTokenAccountsByProgram(pk, program)
		out = append(out, part...)
	}
	return dedupeTokensByMint(out)
}

func fetchTokenAccountsByProgram(ownerPubkey, programID string) []Token {
	params := []interface{}{
		ownerPubkey,
		map[string]string{"programId": programID},
		map[string]string{"encoding": "jsonParsed"},
	}
	var res struct {
		Value []struct {
			Account struct {
				Data json.RawMessage `json:"data"`
			} `json:"account"`
		} `json:"value"`
	}
	if err := callRPC("getTokenAccountsByOwner", params, &res); err != nil {
		return nil
	}
	var out []Token
	for _, v := range res.Value {
		tok := parseParsedTokenAccount(v.Account.Data)
		if tok.Amount <= 0 || tok.Mint == "" {
			continue
		}
		out = append(out, tok)
	}
	return out
}

func parseParsedTokenAccount(data json.RawMessage) Token {
	var wrap struct {
		Parsed *struct {
			Info *struct {
				Mint        string `json:"mint"`
				TokenAmount *struct {
					UIAmount *float64 `json:"uiAmount"`
				} `json:"tokenAmount"`
			} `json:"info"`
		} `json:"parsed"`
	}
	if json.Unmarshal(data, &wrap) != nil || wrap.Parsed == nil || wrap.Parsed.Info == nil {
		return Token{}
	}
	var t Token
	t.Mint = strings.TrimSpace(wrap.Parsed.Info.Mint)
	if wrap.Parsed.Info.TokenAmount != nil && wrap.Parsed.Info.TokenAmount.UIAmount != nil {
		t.Amount = *wrap.Parsed.Info.TokenAmount.UIAmount
	}
	return t
}

func dedupeTokensByMint(in []Token) []Token {
	byMint := make(map[string]float64)
	for _, t := range in {
		byMint[t.Mint] += t.Amount
	}
	out := make([]Token, 0, len(byMint))
	for mint, amt := range byMint {
		if amt <= 0 {
			continue
		}
		out = append(out, Token{Mint: mint, Amount: amt})
	}
	return out
}
