package usecase

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/settlement"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

type accountingTxStub struct {
	port.Tx
	facts  wager.AccountingFacts
	settle func(context.Context, string) error
}

func (s accountingTxStub) LoadAccounting(context.Context, *wager.Transaction) (wager.AccountingFacts, error) {
	return s.facts, nil
}

func (s accountingTxStub) ApplyAccounting(context.Context, *wager.Transaction, wager.AccountingFacts, wager.AccountingDecision, time.Time) error {
	panic("unexpected accounting write")
}
func (s accountingTxStub) SettleByID(c context.Context, id string) error { return s.settle(c, id) }

func TestProcessSettlementLoadsPersistedOutcomeWithoutSecondPosting(t *testing.T) {
	for _, stage := range []string{"success", "settlement failure", "read failure"} {
		t.Run(stage, func(t *testing.T) {
			pending := pathTransaction(t, wager.KindWin)
			persisted := pathTransaction(t, wager.KindWin)
			if err := persisted.ResolveReference("original-bet"); err != nil {
				t.Fatal(err)
			}
			if err := persisted.MarkProcessed(pathMoney(t, 1100), pathTime); err != nil {
				t.Fatal(err)
			}
			var calls []string
			tx := accountingTxStub{Tx: txStub{t: transactionStub{findID: func(_ context.Context, id string) (*wager.Transaction, error) {
				calls = append(calls, "read")
				if id != pending.ID() {
					t.Fatal(id)
				}
				if stage == "read failure" {
					return nil, errPort
				}
				return persisted, nil
			}}}, facts: wager.AccountingFacts{SettlementID: settlementTestID}, settle: func(_ context.Context, id string) error {
				calls = append(calls, "settle")
				if id != settlementTestID {
					t.Fatal(id)
				}
				if stage == "settlement failure" {
					return errPort
				}
				return nil
			}}
			err := pathSubmit(nil).process(context.Background(), tx, pending)
			wantCalls := []string{"settle", "read"}
			if stage == "success" {
				if err != nil || !reflect.DeepEqual(pending.Snapshot(), persisted.Snapshot()) {
					t.Fatal(pending.Snapshot(), err)
				}
			} else {
				if !errors.Is(err, errPort) || pending.Status() != wager.StatusPending {
					t.Fatal(err, pending.Status())
				}
				if stage == "settlement failure" {
					wantCalls = []string{"settle"}
				}
			}
			if !reflect.DeepEqual(calls, wantCalls) {
				t.Fatal(calls)
			}
		})
	}
}

func TestProcessRequiresAccountingAndValidTransitionTime(t *testing.T) {
	uc := pathSubmit(nil)
	// Embedding only the base port deliberately withholds accounting capability.
	if err := uc.process(context.Background(), struct{ port.Tx }{}, pathTransaction(t, wager.KindBet)); err == nil || !strings.Contains(err.Error(), "paired accounting storage") {
		t.Fatal(err)
	}
	tx := pathTransaction(t, wager.KindLoss)
	g := pathWallet(t).Snapshot()
	op := g
	op.ID = "operational"
	uc.clock = clockFunc(func() time.Time { return pathTime.Add(-time.Second) })
	err := uc.process(context.Background(), accountingTxStub{facts: wager.AccountingFacts{Guarantee: g, Operational: op}}, tx)
	if !errors.Is(err, wager.ErrInvalidInput) || tx.Status() != wager.StatusPending {
		t.Fatal(err, tx.Status())
	}
}

func TestPositiveOpeningStopsAtEachFinancialDependency(t *testing.T) {
	for _, stage := range []string{"opening identity", "ledger identity", "ledger append", "processed event", "balance event", "success"} {
		t.Run(stage, func(t *testing.T) {
			var calls []string
			var events []string
			unit := txStub{
				w: walletStub{create: func(_ context.Context, w *wallet.Wallet) error {
					calls = append(calls, "wallet")
					if w.Balance().Minor() != 100 {
						t.Fatal(w.Balance())
					}
					return nil
				}},
				t: transactionStub{insert: func(_ context.Context, tx *wager.Transaction) error {
					calls = append(calls, "opening")
					if tx.Kind() != wager.KindOpening || tx.Status() != wager.StatusProcessed || tx.Amount().Minor() != 100 {
						t.Fatal(tx.Snapshot())
					}
					return nil
				}},
				l: ledgerStub{appendEntry: func(_ context.Context, e wager.LedgerEntry) error {
					calls = append(calls, "ledger")
					if e.Direction() != wager.Credit || e.BalanceBefore().Minor() != 0 || e.BalanceAfter().Minor() != 100 {
						t.Fatal(e)
					}
					if stage == "ledger append" {
						return errPort
					}
					return nil
				}},
				o: outboxStub{add: func(_ context.Context, e event.Outgoing) error {
					calls = append(calls, e.Type())
					events = append(events, e.Type())
					if stage == "processed event" || stage == "balance event" && len(events) == 2 {
						return errPort
					}
					return nil
				}},
			}
			ids := 0
			u := &uowStub{tx: unit}
			uc := NewOpenWallet(u, clockFunc(func() time.Time { return pathTime }), idFunc(func() string {
				ids++
				if stage == "opening identity" && ids == 2 || stage == "ledger identity" && ids == 3 {
					return ""
				}
				return "id"
			}))
			_, err := uc.Execute(context.Background(), OpenWalletInput{PlayerID: pathInput().PlayerID, InitialBalance: pathMoney(t, 100)})
			want := []string{"wallet", "opening", "ledger", event.TypeWagerTransactionProcessed, event.TypeWalletBalanceChanged}
			switch stage {
			case "opening identity":
				want = want[:1]
			case "ledger identity":
				want = want[:2]
			case "ledger append":
				want = want[:3]
			case "processed event":
				want = want[:4]
			}
			if !reflect.DeepEqual(calls, want) {
				t.Fatal("continued after failure", calls, want)
			}
			if stage == "success" {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				expected := errPort
				if stage == "opening identity" || stage == "ledger identity" {
					expected = wager.ErrInvalidInput
				}
				if !errors.Is(err, expected) {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestSubmitExplicitBetRejectsInvalidIdentityAndPersistenceFailures(t *testing.T) {
	for _, stage := range []string{"invalid id", "wrong kind", "missing store", "lock failure", "foreign context", "bind failure"} {
		t.Run(stage, func(t *testing.T) {
			in := pathInput()
			in.BetID = settlementTestID
			var calls []string
			base := txStub{t: transactionStub{
				findKey: func(context.Context, string, string) (*wager.Transaction, error) {
					calls = append(calls, "find")
					return nil, apperr.ErrNotFound
				},
				pending: func(context.Context, *wager.Transaction) (bool, error) {
					calls = append(calls, "insert")
					return true, nil
				},
			}, w: walletStub{get: func(context.Context, string) (*wallet.Wallet, error) { return pathWallet(t), nil }}}
			store := settlementStoreStub{lock: func(_ context.Context, id string) (settlement.Bet, error) {
				calls = append(calls, "lock")
				if id != settlementTestID {
					t.Fatal(id)
				}
				if stage == "lock failure" {
					return settlement.Bet{}, errPort
				}
				b := settlement.Bet{ID: id, ProviderID: in.ProviderID, RoundID: in.RoundID, GameID: in.GameID, Currency: in.Currency, Status: "OPEN"}
				if stage == "foreign context" {
					b.ProviderID = "another provider"
				}
				return b, nil
			}, bind: func(_ context.Context, tid, bid string) error {
				calls = append(calls, "bind")
				if tid != "generated" || bid != settlementTestID {
					t.Fatal(tid, bid)
				}
				return errPort
			}}
			u := &uowStub{tx: settlementTxStub{Tx: base, store: store}}
			want := errPort
			wantCalls := []string{"find", "insert", "lock", "bind"}
			switch stage {
			case "invalid id":
				in.BetID = "bad"
				want = apperr.ErrInvalidInput
				wantCalls = nil
			case "wrong kind":
				in.Kind = "WIN"
				want = apperr.ErrInvalidInput
				wantCalls = nil
			case "missing store":
				u.tx = base
				wantCalls = wantCalls[:2]
			case "lock failure":
				wantCalls = wantCalls[:3]
			case "foreign context":
				want = apperr.ErrInvalidInput
				wantCalls = wantCalls[:3]
			}
			_, err := pathSubmit(u).Execute(context.Background(), in)
			if stage == "missing store" {
				if err == nil || !strings.Contains(err.Error(), "persistence is not configured") {
					t.Fatal(err)
				}
			} else if !errors.Is(err, want) {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(calls, wantCalls) {
				t.Fatal(calls, wantCalls)
			}
		})
	}
}

type bindingInboxStub struct {
	inboxStub
	bind func(context.Context, string, string, string) error
}

func (s bindingInboxStub) BindTransaction(c context.Context, consumer, message, transaction string) error {
	return s.bind(c, consumer, message, transaction)
}

func TestSubmitInboxBindingFailurePreventsCompletion(t *testing.T) {
	in := pathInput()
	in.Source = SourceSQS
	in.InboxMessageID = "message"
	in.InboxHash = "hash"
	m := pathMoney(t, 100)
	hash := wager.PayloadHash(wager.HashInput{ProviderID: in.ProviderID, ExternalTransactionID: in.ExternalTransactionID, PlayerID: in.PlayerID, WalletID: in.WalletID, RoundID: in.RoundID, GameID: in.GameID, Kind: in.Kind, Amount: m.Amount(), Currency: m.Currency()})
	s := pathTransaction(t, wager.KindBet).Snapshot()
	s.PayloadHash = hash
	tx, err := wager.Rehydrate(s)
	if err != nil {
		t.Fatal(err)
	}
	bound := 0
	u := &uowStub{tx: txStub{t: transactionStub{findKey: func(context.Context, string, string) (*wager.Transaction, error) { return tx, nil }}, i: bindingInboxStub{
		inboxStub: inboxStub{begin: func(context.Context, string, string, string, time.Time) (port.InboxState, error) {
			return port.InboxNew, nil
		}},
		bind: func(_ context.Context, consumer, message, id string) error {
			bound++
			if consumer != InboxConsumerName || message != in.InboxMessageID || id != tx.ID() {
				t.Fatal(consumer, message, id)
			}
			return errPort
		},
	}}}
	if _, err = pathSubmit(u).Execute(context.Background(), in); !errors.Is(err, errPort) || bound != 1 {
		t.Fatal(err, bound)
	}
}

func TestSettlementDeliveryRequiresDedicatedExecutor(t *testing.T) {
	u := &uowStub{tx: txStub{}}
	body := []byte(`{"messageId":"message","type":"SettlementRequested","occurredAt":"2026-09-29T12:00:00Z","data":{"settlementId":"` + settlementTestID + `"}}`)
	err := NewConsumeSettlementMessage(pathSubmit(u)).Handle(context.Background(), body)
	if !errors.Is(err, apperr.ErrInvalidInput) || apperr.IsTransient(err) || u.writes != 1 {
		t.Fatal(err, u.writes)
	}
}
