package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

func pathInput() SubmitInput {
	return SubmitInput{Source: SourceHTTP, AuthorizedProviderID: "p", ProviderID: "p", ExternalTransactionID: "external", IdempotencyKey: "key", PlayerID: "00000000-0000-4000-8000-000000000001", WalletID: "00000000-0000-4000-8000-000000000002", RoundID: "round", GameID: "game", Kind: "BET", Amount: "1.00", Currency: "BRL"}
}

func TestSubmitInputAndLookupFailures(t *testing.T) {
	for name, change := range map[string]func(*SubmitInput){"unknown source": func(i *SubmitInput) { i.Source = "unknown" }, "missing inbox": func(i *SubmitInput) { i.Source = SourceSQS }, "invalid wallet": func(i *SubmitInput) { i.WalletID = "bad" }} {
		t.Run(name, func(t *testing.T) {
			u := &uowStub{}
			in := pathInput()
			change(&in)
			if _, e := pathSubmit(u).Execute(context.Background(), in); !errors.Is(e, apperr.ErrInvalidInput) || u.writes != 0 {
				t.Fatal(e, u.writes)
			}
		})
	}
	for _, stage := range []string{"inbox", "lookup", "wallet", "insert", "second lookup", "racing insert", "transient conflict"} {
		t.Run(stage, func(t *testing.T) {
			in := pathInput()
			calls := 0
			var inserted *wager.Transaction
			fail := errPort
			if stage == "transient conflict" {
				fail = apperr.Transient(apperr.ErrConcurrentModification)
			}
			repo := transactionStub{
				findKey: func(context.Context, string, string) (*wager.Transaction, error) {
					calls++
					if stage == "lookup" || stage == "transient conflict" || (stage == "second lookup" && calls == 2) {
						return nil, fail
					}
					if calls == 2 {
						return inserted, nil
					}
					return nil, apperr.ErrNotFound
				},
				pending: func(_ context.Context, tx *wager.Transaction) (bool, error) {
					if stage == "insert" {
						return false, fail
					}
					inserted = tx
					if e := tx.MarkProcessed(pathMoney(t, 900), pathTime); e != nil {
						t.Fatal(e)
					}
					return false, nil
				},
			}
			u := &uowStub{tx: txStub{t: repo, w: walletStub{get: func(context.Context, string) (*wallet.Wallet, error) {
				if stage == "wallet" {
					return nil, fail
				}
				return pathWallet(t), nil
			}}, i: inboxStub{begin: func(context.Context, string, string, string, time.Time) (port.InboxState, error) {
				return port.InboxNew, fail
			}}}}
			if stage == "inbox" {
				in.Source = SourceSQS
				in.InboxMessageID = "message"
				in.InboxHash = "hash"
			}
			m := &metricsSpy{}
			uc := pathSubmit(u)
			uc.metrics = m
			r, e := uc.Execute(context.Background(), in)
			if stage == "racing insert" {
				if e != nil || !r.IdempotentReplay || r.Balance.Minor() != 900 || r.TransactionID != inserted.ID() {
					t.Fatal(r, e)
				}
			} else if !errors.Is(e, fail) {
				t.Fatal(e)
			}
			if stage == "transient conflict" && (m.conflicts != 1 || len(m.retries) != 1 || m.retries[0] != "submit") {
				t.Fatal("unobserved retry/conflict", m)
			}
		})
	}
	if e := NewConsumeWagerMessage(nil).Handle(context.Background(), []byte(`{"messageId":"m","type":"WagerTransactionRequested","data":{}}`)); !errors.Is(e, apperr.ErrInvalidMessage) {
		t.Fatal("missing timestamp accepted", e)
	}
}

func TestProcessingPropagatesDomainAndPortFailures(t *testing.T) {
	ctx := context.Background()
	for _, stage := range []string{"wallet", "reference", "reversal lookup", "reference already resolved", "invalid wallet time", "ledger identity", "processed transition"} {
		t.Run(stage, func(t *testing.T) {
			tx := pathTransaction(t, wager.KindBet)
			w := pathWallet(t)
			ref := pathTransaction(t, wager.KindBet)
			rs := ref.Snapshot()
			rs.ID = "original"
			rs.ExternalID = "bet"
			ref, e := wager.Rehydrate(rs)
			if e != nil {
				t.Fatal(e)
			}
			if e = ref.MarkProcessed(pathMoney(t, 900), pathTime); e != nil {
				t.Fatal(e)
			}
			expected := errPort
			if stage == "reference" || stage == "reversal lookup" || stage == "reference already resolved" {
				tx = pathTransaction(t, wager.KindRefund)
			}
			if stage == "reference already resolved" {
				if e := tx.ResolveReference("different"); e != nil {
					t.Fatal(e)
				}
				expected = wager.ErrInvalidInput
			}
			if stage == "invalid wallet time" {
				s := w.Snapshot()
				s.UpdatedAt = pathTime.Add(time.Minute)
				w, e = wallet.Rehydrate(s)
				if e != nil {
					t.Fatal(e)
				}
				expected = wallet.ErrInvalidWallet
			}
			if stage == "ledger identity" {
				expected = wager.ErrInvalidInput
			}
			if stage == "processed transition" {
				tx = pathTransaction(t, wager.KindLoss)
				if e := tx.Reject(wager.FailInvalidInput, pathTime); e != nil {
					t.Fatal(e)
				}
				expected = wager.ErrTerminalState
			}
			repo := transactionStub{findExternal: func(context.Context, string, string) (*wager.Transaction, error) {
				if stage == "reference" {
					return nil, errPort
				}
				return ref, nil
			}, reversal: func(context.Context, string) (bool, error) {
				if stage == "reversal lookup" {
					return false, errPort
				}
				return false, nil
			}}
			unit := txStub{w: walletStub{get: func(context.Context, string) (*wallet.Wallet, error) {
				if stage == "wallet" {
					return nil, errPort
				}
				return w, nil
			}}, t: repo}
			uc := pathSubmit(nil)
			if stage == "ledger identity" {
				uc.ids = idFunc(func() string { return "" })
			}
			if e = uc.process(ctx, unit, tx); !errors.Is(e, expected) {
				t.Fatal(stage, e)
			}
		})
	}
	for _, op := range []string{"reject terminal", "reject update", "wait invalid", "wait update"} {
		t.Run(op, func(t *testing.T) {
			tx := pathTransaction(t, wager.KindRefund)
			uc := pathSubmit(nil)
			expected := errPort
			unit := txStub{t: transactionStub{update: func(context.Context, *wager.Transaction) error { return errPort }}}
			if op == "reject terminal" {
				if e := tx.Reject(wager.FailInvalidInput, pathTime); e != nil {
					t.Fatal(e)
				}
				expected = wager.ErrTerminalState
			}
			if op == "wait invalid" {
				uc.clock = clockFunc(func() time.Time { return time.Time{} })
				expected = wager.ErrInvalidInput
			}
			var e error
			if op == "reject terminal" || op == "reject update" {
				e = uc.reject(ctx, unit, tx, wager.FailInvalidInput)
			} else {
				e = uc.waitReference(ctx, unit, tx)
			}
			if !errors.Is(e, expected) {
				t.Fatal(op, e)
			}
		})
	}
	if e := addEvent(ctx, txStub{}, event.Outgoing{}, errPort); !errors.Is(e, errPort) {
		t.Fatal(e)
	}
}
