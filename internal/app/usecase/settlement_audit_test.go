package usecase

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/settlement"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

const settlementTestID = "aaaaaaaa-0000-4000-8000-000000000001"

type auditTxStub struct {
	port.Tx
	store port.SettlementAuditStore
}

func (s auditTxStub) SettlementAudit() port.SettlementAuditStore { return s.store }

type auditStub struct {
	audit   func(context.Context, string) (port.SettlementAudit, error)
	lock    func(context.Context, string) (port.SettlementRecord, settlement.ReversalFacts, error)
	reverse func(context.Context, string, time.Time) error
}

func (s auditStub) AuditSettlement(c context.Context, id string) (port.SettlementAudit, error) {
	return s.audit(c, id)
}

func (s auditStub) LockReversal(c context.Context, id string) (port.SettlementRecord, settlement.ReversalFacts, error) {
	return s.lock(c, id)
}

func (s auditStub) ReverseSettlement(c context.Context, id string, at time.Time) error {
	return s.reverse(c, id, at)
}

func TestSettlementAuditUsesSnapshotAndPreservesHistory(t *testing.T) {
	ctx := context.Background()
	want := port.SettlementAudit{
		SettlementRecord: port.SettlementRecord{ID: settlementTestID, Status: "PROCESSED"},
		Payments:         []port.SettlementPayment{{ID: "payment", ReferenceID: "bet", Money: pathMoney(t, 100)}},
		Postings:         []port.SettlementPosting{{ID: "entry", JournalID: "journal", AccountID: "operational", Direction: "DEBIT", Money: pathMoney(t, 100), Sequence: 7}},
	}
	for _, stage := range []string{"success", "invalid id", "missing storage", "read failure"} {
		t.Run(stage, func(t *testing.T) {
			calls := 0
			var expected error
			store := auditStub{audit: func(_ context.Context, id string) (port.SettlementAudit, error) {
				calls++
				if id != settlementTestID {
					t.Fatal(id)
				}
				return want, expected
			}}
			u := &uowStub{tx: auditTxStub{store: store}}
			id := strings.ToUpper(settlementTestID)
			if stage == "invalid id" {
				id = "bad"
				expected = apperr.ErrInvalidInput
			}
			if stage == "missing storage" {
				u.tx = txStub{}
			}
			if stage == "read failure" {
				expected = errPort
			}
			out, err := pathSubmit(u).Settlements().Get(ctx, id)
			if stage == "missing storage" {
				if err == nil || !strings.Contains(err.Error(), "storage missing") {
					t.Fatal(err)
				}
			} else if !errors.Is(err, expected) {
				t.Fatal(err)
			}
			if stage == "success" {
				if !reflect.DeepEqual(out, want) {
					t.Fatal(out)
				}
			} else if !reflect.DeepEqual(out, port.SettlementAudit{}) {
				t.Fatal("partial audit exposed", out)
			}
			wantCalls := 1
			if stage == "invalid id" || stage == "missing storage" {
				wantCalls = 0
			}
			wantReads := 1
			if stage == "invalid id" {
				wantReads = 0
			}
			if calls != wantCalls || u.reads != wantReads || u.writes != 0 {
				t.Fatal(calls, u.reads, u.writes)
			}
		})
	}
}

func auditReversalFacts(t *testing.T) settlement.ReversalFacts {
	t.Helper()
	f := settlement.ReversalFacts{Status: "PROCESSED", Accounts: map[string]wallet.Snapshot{}}
	for i := 0; i < 2; i++ {
		id := fmt.Sprintf("payment-%d", i)
		g := fmt.Sprintf("g-%d", i)
		op := fmt.Sprintf("op-%d", i)
		payment := pathTransaction(t, wager.KindWin).Snapshot()
		payment.ID = id
		payment.ExternalID = id
		payment.WalletID = g
		payment.Status = wager.StatusProcessed
		f.Payments = append(f.Payments, payment)
		gw, _ := wallet.New(g, "player", pathMoney(t, 100), pathTime)
		ow, _ := wallet.New(op, "player", pathMoney(t, 0), pathTime)
		f.Accounts[g] = gw.Snapshot()
		f.Accounts[op] = ow.Snapshot()
		f.Journals = append(f.Journals, settlement.ReversalJournal{DebitAccountID: op, CreditAccountID: g, Amount: pathMoney(t, 100)})
	}
	return f
}

func TestSettlementReversalCommandsAreAtomicAndReplayDoesNotWrite(t *testing.T) {
	for _, stage := range []string{"success", "replay", "invalid id", "missing storage", "lock failure", "invalid history", "invalid payment", "invalid reference", "second insert failure", "reverse failure"} {
		t.Run(stage, func(t *testing.T) {
			facts := auditReversalFacts(t)
			record := port.SettlementRecord{ID: settlementTestID, BetID: "bet", ResultID: "result", Status: "PROCESSED", Hash: "hash"}
			var expected error
			switch stage {
			case "replay":
				record.Status = "REVERSED"
				facts = settlement.ReversalFacts{}
			case "invalid id":
				expected = apperr.ErrInvalidInput
			case "lock failure", "second insert failure", "reverse failure":
				expected = errPort
			case "invalid history":
				facts.Status = "CONFIRMED"
				expected = settlement.ErrReversalState
			case "invalid payment":
				facts.Payments[1].ProviderID = ""
				expected = wager.ErrInvalidInput
			case "invalid reference":
				facts.Payments[1].ID = ""
				expected = wager.ErrInvalidInput
			}
			var staged, committed []wager.Snapshot
			var calls []string
			ids := 0
			store := auditStub{lock: func(_ context.Context, id string) (port.SettlementRecord, settlement.ReversalFacts, error) {
				calls = append(calls, "lock")
				if id != settlementTestID {
					t.Fatal(id)
				}
				if stage == "lock failure" {
					return record, facts, errPort
				}
				return record, facts, nil
			}, reverse: func(_ context.Context, id string, at time.Time) error {
				calls = append(calls, "reverse")
				if id != record.ID || at != pathTime || len(staged) != 2 {
					t.Fatal("incomplete reversal", id, at, staged)
				}
				if stage == "reverse failure" {
					return errPort
				}
				return nil
			}}
			tx := auditTxStub{Tx: txStub{t: transactionStub{insert: func(_ context.Context, tx *wager.Transaction) error {
				calls = append(calls, "insert")
				if stage == "second insert failure" && len(staged) == 1 {
					return errPort
				}
				staged = append(staged, tx.Snapshot())
				return nil
			}}}, store: store}
			u := &uowStub{tx: tx}
			if stage == "missing storage" {
				u.tx = txStub{}
			}
			// This fake models only the UnitOfWork commit contract. The domain code
			// validates the actual accounting history; no ledger algorithm lives here.
			u.do = func(c context.Context, f func(context.Context, port.Tx) error) error {
				err := f(c, u.tx)
				if err == nil {
					committed = append(committed, staged...)
				}
				return err
			}
			uc := NewSettlements(u, clockFunc(func() time.Time { return pathTime }), idFunc(func() string { ids++; return fmt.Sprintf("reversal-%d", ids) }))
			id := strings.ToUpper(settlementTestID)
			if stage == "invalid id" {
				id = "bad"
			}
			out, err := uc.Reverse(context.Background(), id)
			if stage == "missing storage" {
				if err == nil || !strings.Contains(err.Error(), "storage missing") {
					t.Fatal(err)
				}
			} else if !errors.Is(err, expected) {
				t.Fatalf("got %v, want %v", err, expected)
			}
			if stage == "success" || stage == "replay" {
				record.Status = "REVERSED"
				if out != record {
					t.Fatal(out)
				}
			} else if out != (port.SettlementRecord{}) || len(committed) != 0 {
				t.Fatal("failed operation reported success", out, committed)
			}
			switch stage {
			case "success":
				if !reflect.DeepEqual(calls, []string{"lock", "insert", "insert", "reverse"}) || len(committed) != 2 {
					t.Fatal(calls, committed)
				}
				for i, s := range committed {
					p := facts.Payments[i]
					external := "settlement-reversal:" + s.ID
					wantHash := wager.PayloadHash(wager.HashInput{ProviderID: p.ProviderID, ExternalTransactionID: external, PlayerID: p.PlayerID, WalletID: p.WalletID, RoundID: p.RoundID, GameID: p.GameID, Kind: "ROLLBACK", Amount: p.Amount.Amount(), Currency: p.Amount.Currency(), ReferenceExternalTransactionID: p.ExternalID})
					if s.Kind != wager.KindRollback || s.Status != wager.StatusPending || s.ReferenceID != p.ID || s.ReferenceExternalID != p.ExternalID || s.Amount != p.Amount || s.WalletID != p.WalletID || s.ProviderID != p.ProviderID || s.PlayerID != p.PlayerID || s.RoundID != p.RoundID || s.GameID != p.GameID || s.ExternalID != external || s.IdempotencyKey != external || s.PayloadHash != wantHash || s.CorrelationID != record.ID {
						t.Fatal("incorrect compensating command", s)
					}
				}
			case "replay":
				if ids != 0 || len(committed) != 0 || !reflect.DeepEqual(calls, []string{"lock"}) {
					t.Fatal("replay wrote new compensation", ids, calls)
				}
			}
			if err != nil && stage != "reverse failure" {
				for _, c := range calls {
					if c == "reverse" {
						t.Fatal("executed incomplete reversal")
					}
				}
			}
			if u.reads != 0 || (stage == "invalid id" && u.writes != 0) {
				t.Fatal("incorrect transaction boundary")
			}
		})
	}
}
