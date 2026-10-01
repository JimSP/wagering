package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/google/uuid"
)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Now().UTC() }

type testIDs struct{}

func (testIDs) NewID() string { return uuid.NewString() }

type replayRepo struct {
	port.TransactionRepository
	t *wager.Transaction
}

func (r replayRepo) FindByIdempotencyKey(context.Context, string, string) (*wager.Transaction, error) {
	return r.t, nil
}

type replayTx struct {
	port.Tx
	r replayRepo
}

func (t replayTx) Transactions() port.TransactionRepository { return t.r }

type replayUOW struct {
	port.UnitOfWork
	t     port.Tx
	calls int
}

func (u *replayUOW) Do(c context.Context, f func(context.Context, port.Tx) error) error {
	u.calls++
	return f(c, u.t)
}

func TestPersistentResultReplayAndPayloadConflict(t *testing.T) {
	in := SubmitInput{Source: SourceHTTP, AuthorizedProviderID: "p", ProviderID: "p", ExternalTransactionID: "ext", IdempotencyKey: "received-key", PlayerID: uuid.NewString(), WalletID: uuid.NewString(), RoundID: "round", GameID: "game", Kind: "BET", Amount: "01.00", Currency: "BRL"}
	hash := wager.PayloadHash(wager.HashInput{ProviderID: in.ProviderID, ExternalTransactionID: in.ExternalTransactionID, PlayerID: in.PlayerID, WalletID: in.WalletID, RoundID: in.RoundID, GameID: in.GameID, Kind: in.Kind, Amount: "1.00", Currency: in.Currency})
	amount, _ := money.Parse("1.00", "BRL")
	tx, e := wager.NewExternal(wager.ExternalParams{ID: uuid.NewString(), ProviderID: in.ProviderID, ExternalID: in.ExternalTransactionID, IdempotencyKey: in.IdempotencyKey, PayloadHash: hash, PlayerID: in.PlayerID, WalletID: in.WalletID, RoundID: in.RoundID, GameID: in.GameID, Kind: wager.KindBet, Amount: amount}, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	balance, _ := money.Parse("99.00", "BRL")
	if e = tx.MarkProcessed(balance, time.Now()); e != nil {
		t.Fatal(e)
	}
	uow := &replayUOW{t: replayTx{r: replayRepo{t: tx}}}
	uc := NewSubmitTransaction(uow, fixedClock{}, testIDs{}, metrics.New())
	r, e := uc.Execute(context.Background(), in)
	if e != nil || !r.IdempotentReplay || r.Balance.Amount() != "99.00" {
		t.Fatal(r, e)
	}
	in.Amount = "2.00"
	if _, e = uc.Execute(context.Background(), in); !errors.Is(e, apperr.ErrIdempotencyConflict) {
		t.Fatal(e)
	}
	calls := uow.calls
	in.AuthorizedProviderID = "other"
	if _, e = uc.Execute(context.Background(), in); !errors.Is(e, apperr.ErrForbidden) || uow.calls != calls {
		t.Fatal("authorization must precede replay", e)
	}
}
