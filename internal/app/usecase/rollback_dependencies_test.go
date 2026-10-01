package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/settlement"
	"github.com/alexandre/wagering/internal/domain/wager"
)

type rollbackTxStub struct {
	port.Tx
	lock     func(context.Context, *wager.Transaction) (port.RollbackContext, error)
	payments func(context.Context, string) ([]wager.Snapshot, error)
	execute  func(context.Context, string, time.Time) error
	scope    func(context.Context, func() error) error
	load     func(context.Context, *wager.Transaction) (wager.AccountingFacts, error)
	apply    func(context.Context, *wager.Transaction, wager.AccountingFacts, wager.AccountingDecision, time.Time) error
	audit    port.SettlementAuditStore
}

func (s rollbackTxStub) LockRollbackContext(c context.Context, t *wager.Transaction) (port.RollbackContext, error) {
	return s.lock(c, t)
}

func (s rollbackTxStub) DependentPayments(c context.Context, id string) ([]wager.Snapshot, error) {
	return s.payments(c, id)
}

func (s rollbackTxStub) ExecuteSettlementAt(c context.Context, id string, at time.Time) error {
	return s.execute(c, id, at)
}

func (s rollbackTxStub) ReversalScope(c context.Context, fn func() error) error {
	return s.scope(c, fn)
}

func (s rollbackTxStub) LoadAccounting(c context.Context, t *wager.Transaction) (wager.AccountingFacts, error) {
	return s.load(c, t)
}

func (s rollbackTxStub) ApplyAccounting(c context.Context, t *wager.Transaction, f wager.AccountingFacts, d wager.AccountingDecision, at time.Time) error {
	return s.apply(c, t, f, d, at)
}

func (s rollbackTxStub) SettleByID(context.Context, string) error {
	panic("unexpected settlement shortcut")
}
func (s rollbackTxStub) SettlementAudit() port.SettlementAuditStore { return s.audit }

func TestRollbackDependenciesAreOrderedAtomicAndRestoreRootAfterFailure(t *testing.T) {
	for _, stage := range []string{"success", "confirmed", "lock", "execute", "reverse", "payments", "invalid child", "insert", "child load", "child reject", "root load", "root reject", "release"} {
		t.Run(stage, func(t *testing.T) {
			root := pathTransaction(t, wager.KindRollback)
			before := root.Snapshot()
			bet := pathTransaction(t, wager.KindBet).Snapshot()
			bet.ID = "bet"
			bet.Status = wager.StatusProcessed
			win := pathTransaction(t, wager.KindWin).Snapshot()
			win.ID = "win"
			win.Status = wager.StatusProcessed
			g := pathWallet(t).Snapshot()
			op := g
			op.ID = "operational"
			var writes []string
			var sequence []string
			locks := 0
			inScope := false
			unit := rollbackTxStub{}
			unit.Tx = txStub{t: transactionStub{insert: func(_ context.Context, tx *wager.Transaction) error {
				sequence = append(sequence, "insert child")
				if tx.Snapshot().CorrelationID != root.ID() || tx.ReferenceExternalID() != win.ExternalID {
					t.Fatal(tx.Snapshot())
				}
				if stage == "insert" {
					return errPort
				}
				writes = append(writes, "child inserted")
				return nil
			}, update: func(_ context.Context, tx *wager.Transaction) error {
				writes = append(writes, string(tx.Status()))
				return nil
			}}, o: outboxStub{add: func(_ context.Context, e event.Outgoing) error { writes = append(writes, e.Type()); return nil }}}
			unit.lock = func(_ context.Context, tx *wager.Transaction) (port.RollbackContext, error) {
				locks++
				if stage == "lock" {
					return port.RollbackContext{}, errPort
				}
				if locks > 1 {
					return port.RollbackContext{}, nil
				}
				d := port.RollbackContext{HasPayments: true}
				if stage == "confirmed" || stage == "execute" || stage == "reverse" {
					d.SettlementID = settlementTestID
					d.SettlementStatus = "CONFIRMED"
				}
				return d, nil
			}
			unit.scope = func(_ context.Context, fn func() error) error {
				inScope = true
				checkpoint := len(writes)
				err := fn()
				inScope = false
				if stage == "release" {
					err = errPort
				}
				if err != nil {
					writes = writes[:checkpoint]
				}
				return err
			}
			unit.execute = func(_ context.Context, id string, at time.Time) error {
				sequence = append(sequence, "execute")
				if id != settlementTestID || at != pathTime {
					t.Fatal(id, at)
				}
				if stage == "execute" {
					return errPort
				}
				return nil
			}
			unit.audit = auditStub{lock: func(context.Context, string) (port.SettlementRecord, settlement.ReversalFacts, error) {
				sequence = append(sequence, "reverse")
				if stage == "reverse" {
					return port.SettlementRecord{}, settlement.ReversalFacts{}, errPort
				}
				return port.SettlementRecord{Status: "REVERSED"}, settlement.ReversalFacts{}, nil
			}}
			unit.payments = func(_ context.Context, ref string) ([]wager.Snapshot, error) {
				sequence = append(sequence, "payments")
				if ref != "bet" {
					t.Fatal(ref)
				}
				if stage == "payments" {
					return nil, errPort
				}
				return []wager.Snapshot{win}, nil
			}
			unit.load = func(_ context.Context, tx *wager.Transaction) (wager.AccountingFacts, error) {
				child := tx.ID() != root.ID()
				f := wager.AccountingFacts{Guarantee: g, Operational: op, Reference: &bet, BetID: "b", BetStatus: "CLOSED", CommitmentID: "c", Remaining: 100, OriginalJournalID: "journal"}
				if child {
					f.Reference = &win
					if stage == "child load" {
						return f, errPort
					}
					if stage == "child reject" {
						f.Guarantee.Balance = pathMoney(t, 0)
					}
				}
				if !child && inScope {
					if stage == "root load" {
						return f, errPort
					}
					if stage == "root reject" {
						f.Remaining = 0
					}
				}
				return f, nil
			}
			unit.apply = func(_ context.Context, tx *wager.Transaction, _ wager.AccountingFacts, _ wager.AccountingDecision, _ time.Time) error {
				if !inScope {
					t.Fatal("financial write outside scope")
				}
				sequence = append(sequence, "apply "+tx.ID())
				writes = append(writes, tx.ID())
				return nil
			}
			uc := pathSubmit(nil)
			if stage == "invalid child" {
				ids := 0
				uc.ids = idFunc(func() string {
					ids++
					if ids == 1 {
						return ""
					}
					return "event"
				})
			}
			err := uc.process(context.Background(), unit, root)
			switch stage {
			case "success", "confirmed":
				if err != nil || root.Status() != wager.StatusProcessed {
					t.Fatal(root.Snapshot(), err)
				}
				want := []string{"payments", "insert child", "apply generated", "apply tx"}
				if stage == "confirmed" {
					want = append([]string{"execute", "reverse"}, want...)
				}
				if !reflect.DeepEqual(sequence, want) {
					t.Fatal(sequence, want)
				}
			case "child reject", "root reject", "invalid child":
				code := wager.FailReversalInsufficientFunds
				if stage == "invalid child" {
					code = wager.FailInvalidInput
				}
				if err != nil || root.Status() != wager.StatusRejected || root.Snapshot().FailureCode != code {
					t.Fatal(root.Snapshot(), err)
				}
				if !reflect.DeepEqual(writes, []string{"REJECTED", event.TypeWagerTransactionRejected}) {
					t.Fatal("partial effects", writes)
				}
			default:
				if !errors.Is(err, errPort) || !reflect.DeepEqual(root.Snapshot(), before) || len(writes) != 0 {
					t.Fatal(root.Snapshot(), err, writes)
				}
			}
		})
	}
}
