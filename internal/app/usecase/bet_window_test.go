package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/settlement"
)

func TestConfiguredWindowIsPersistedAtBetCreation(t *testing.T) {
	var saved settlement.Bet
	u := &uowStub{tx: settlementTxStub{store: settlementStoreStub{create: func(_ context.Context, b settlement.Bet) error { saved = b; return nil }}}}
	base := pathSubmit(u)
	configured := newConfiguredSubmitTransaction(u, base.clock, base.ids, base.metrics, BettingWindow(17*time.Second))
	b := settlement.Bet{ID: settlementTestID, ProviderID: "p", RoundID: "r", GameID: "g", Currency: "BRL", BettingWindowSeconds: 999}
	if err := configured.Settlements().Create(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	if saved.BettingWindowSeconds != 17 || !saved.CreatedAt.Equal(pathTime) {
		t.Fatalf("persisted %+v", saved)
	}
}

func TestDefaultWindowIsPersistedAtBetCreation(t *testing.T) {
	var saved settlement.Bet
	u := &uowStub{tx: settlementTxStub{store: settlementStoreStub{create: func(_ context.Context, b settlement.Bet) error { saved = b; return nil }}}}
	submit := pathSubmit(u)
	b := settlement.Bet{ID: settlementTestID, ProviderID: "p", RoundID: "r", GameID: "g", Currency: "BRL", BettingWindowSeconds: 999}
	if err := submit.Settlements().Create(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	if saved.BettingWindowSeconds != 300 || !saved.CreatedAt.Equal(pathTime) {
		t.Fatalf("persisted default: %+v", saved)
	}
}
