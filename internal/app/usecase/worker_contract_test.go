package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

func TestMessageEnvelopeRejectsEachRequiredField(t *testing.T) {
	for _, field := range []string{"messageId", "type", "idempotencyKey"} {
		t.Run(field, func(t *testing.T) {
			in := WagerTransactionRequested{MessageID: "m", Type: "WagerTransactionRequested", OccurredAt: pathTime}
			valid := pathInput()
			in.Data.IdempotencyKey = valid.IdempotencyKey
			in.Data.ProviderID = valid.ProviderID
			in.Data.ExternalTransactionID = valid.ExternalTransactionID
			in.Data.PlayerID = valid.PlayerID
			in.Data.WalletID = valid.WalletID
			in.Data.RoundID = valid.RoundID
			in.Data.GameID = valid.GameID
			in.Data.Kind = valid.Kind
			in.Data.Money.Amount = valid.Amount
			in.Data.Money.Currency = valid.Currency
			switch field {
			case "messageId":
				in.MessageID = ""
			case "type":
				in.Type = "unknown"
			case "idempotencyKey":
				in.Data.IdempotencyKey = ""
			}
			body, err := json.Marshal(in)
			if err != nil {
				t.Fatal(err)
			}
			u := &uowStub{}
			err = NewConsumeWagerMessage(pathSubmit(u)).Handle(context.Background(), body)
			if !errors.Is(err, apperr.ErrInvalidMessage) || u.writes != 0 {
				t.Fatalf("err=%v writes=%d", err, u.writes)
			}
		})
	}
}

func TestReferenceWorkerBatchCancellationAndPermanentFailureContinuation(t *testing.T) {
	for _, mode := range []string{"full batch", "cancelled initially", "cancel after first"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancelled initially" {
				cancel()
			}
			claims, updates := 0, 0
			var current *wager.Transaction
			repo := transactionStub{claim: func(_ context.Context, now time.Time, limit int) ([]*wager.Transaction, error) {
				claims++
				if claims > 21 {
					return nil, errors.New("exceeded contract batch bound")
				}
				if limit != 1 || now != pathTime {
					t.Fatalf("claim limit=%d time=%v", limit, now)
				}
				s := pathTransaction(t, wager.KindRefund).Snapshot()
				s.ID = fmt.Sprint("tx-", claims)
				current, _ = wager.Rehydrate(s)
				return []*wager.Transaction{current}, nil
			}, findID: func(_ context.Context, id string) (*wager.Transaction, error) {
				if id != current.ID() {
					t.Fatal(id)
				}
				return current, nil
			}, update: func(_ context.Context, tx *wager.Transaction) error {
				updates++
				if tx.Status() != wager.StatusFailed || tx.FailureCode() != wager.FailInternalPermanent {
					t.Fatal(tx.Snapshot())
				}
				if mode == "cancel after first" {
					cancel()
				}
				return nil
			}}
			u := &uowStub{tx: txStub{t: repo, w: walletStub{get: func(context.Context, string) (*wallet.Wallet, error) { return nil, apperr.Permanent(errPort) }}}}
			m := &metricsSpy{}
			n, err := NewProcessPendingReferences(u, clockFunc(func() time.Time { return pathTime }), m, pathSubmit(u)).RunOnce(ctx)
			want := 20
			if mode == "cancelled initially" {
				want = 0
			}
			if mode == "cancel after first" {
				want = 1
			}
			if n != want || claims != want || updates != want || len(m.results) != want {
				t.Fatalf("n=%d claims=%d updates=%d metrics=%v err=%v", n, claims, updates, m.results, err)
			}
			if mode == "full batch" {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			for _, v := range m.results {
				if v != "REFUND:FAILED" {
					t.Fatal(v)
				}
			}
		})
	}
}

func TestEmptyReferenceQueueReturnsWithoutRepeatedClaims(t *testing.T) {
	claims := 0
	repo := transactionStub{claim: func(context.Context, time.Time, int) ([]*wager.Transaction, error) {
		claims++
		if claims > 1 {
			t.Fatal("empty queue was queried again without progress")
		}
		return nil, nil
	}}
	u := &uowStub{tx: txStub{t: repo}}
	n, e := NewProcessPendingReferences(u, clockFunc(func() time.Time { return pathTime }), &metricsSpy{}, pathSubmit(u)).RunOnce(context.Background())
	if n != 0 || e != nil || claims != 1 {
		t.Fatalf("n=%d error=%v claims=%d", n, e, claims)
	}
}
