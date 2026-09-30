package wager_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/alexandre/wagering/internal/domain/wager"
)

func TestBusinessIdentityIncludesEveryFinancialFieldInCanonicalOrder(t *testing.T) {
	input := wager.HashInput{ProviderID: "p", ExternalTransactionID: "ext", PlayerID: "player", WalletID: "wallet", RoundID: "round", GameID: "game", Kind: "BET", Amount: "25.00", Currency: "BRL"}
	// Literal oracle: the test never constructs its expected JSON using the code
	// path that is being tested. Key order includes the nested money object.
	canonical := `{"externalTransactionId":"ext","gameId":"game","kind":"BET","money":{"amount":"25.00","currency":"BRL"},"playerId":"player","providerId":"p","roundId":"round","walletId":"wallet"}`
	sum := sha256.Sum256([]byte(canonical))
	want := hex.EncodeToString(sum[:])
	if got := wager.PayloadHash(input); got != want {
		t.Fatalf("canonical hash: got %s want %s", got, want)
	}
	referenced := input
	referenced.ReferenceExternalTransactionID = "original"
	withReference := `{"externalTransactionId":"ext","gameId":"game","kind":"BET","money":{"amount":"25.00","currency":"BRL"},"playerId":"player","providerId":"p","referenceExternalTransactionId":"original","roundId":"round","walletId":"wallet"}`
	referenceSum := sha256.Sum256([]byte(withReference))
	if got := wager.PayloadHash(referenced); got != hex.EncodeToString(referenceSum[:]) {
		t.Fatal("reference canonical order", got)
	}

	for name, change := range map[string]func(*wager.HashInput){"provider": func(i *wager.HashInput) { i.ProviderID = "other" }, "external": func(i *wager.HashInput) { i.ExternalTransactionID = "other" }, "player": func(i *wager.HashInput) { i.PlayerID = "other" }, "wallet": func(i *wager.HashInput) { i.WalletID = "other" }, "round": func(i *wager.HashInput) { i.RoundID = "other" }, "game": func(i *wager.HashInput) { i.GameID = "other" }, "kind": func(i *wager.HashInput) { i.Kind = "WIN" }, "amount": func(i *wager.HashInput) { i.Amount = "25.01" }, "currency": func(i *wager.HashInput) { i.Currency = "USD" }, "reference": func(i *wager.HashInput) { i.ReferenceExternalTransactionID = "original" }} {
		t.Run(name, func(t *testing.T) {
			changed := input
			change(&changed)
			if wager.PayloadHash(changed) == want {
				t.Fatal("financial meaning not part of identity")
			}
		})
	}
}
