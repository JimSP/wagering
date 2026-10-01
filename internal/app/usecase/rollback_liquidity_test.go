package usecase

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/settlement"
	"github.com/alexandre/wagering/internal/domain/wager"
)

type liquidityTxStub struct {
	rollbackTxStub
	liquidity func(context.Context, string, string, int64, time.Time) (port.RollbackLiquidityPlan, error)
}

func (s liquidityTxStub) LoadRollbackLiquidity(c context.Context, w, b string, n int64, at time.Time) (port.RollbackLiquidityPlan, error) {
	return s.liquidity(c, w, b, n, at)
}

func TestLiquidityRecoveryCommitsOnlyWithSuccessfulOriginalReversal(t *testing.T) {
	for _, stage := range []string{"recover", "available", "expired then eligible", "accumulate existing balance", "waiting", "window elapsed", "insufficient", "lookup", "child insert", "root apply", "root rejected", "release"} {
		t.Run(stage, func(t *testing.T) {
			root := pathTransaction(t, wager.KindRollback)
			before := root.Snapshot()
			win := pathTransaction(t, wager.KindWin).Snapshot()
			win.ID = "original"
			win.Status = wager.StatusProcessed
			bet := pathTransaction(t, wager.KindBet).Snapshot()
			bet.ID = "other"
			bet.Status = wager.StatusProcessed
			available := int64(0)
			if stage == "accumulate existing balance" {
				available = 40
				bet.Amount = pathMoney(t, 60)
			}
			var writes []string
			var sequence []string
			base := rollbackTxStub{}
			base.Tx = txStub{t: transactionStub{insert: func(_ context.Context, tx *wager.Transaction) error {
				sequence = append(sequence, "child")
				if stage == "child insert" {
					return errPort
				}
				writes = append(writes, tx.ID())
				return nil
			}, update: func(_ context.Context, tx *wager.Transaction) error {
				writes = append(writes, string(tx.Status()))
				return nil
			}}, o: outboxStub{add: func(_ context.Context, e event.Outgoing) error { writes = append(writes, e.Type()); return nil }}}
			base.lock = func(context.Context, *wager.Transaction) (port.RollbackContext, error) {
				return port.RollbackContext{}, nil
			}
			base.scope = func(_ context.Context, fn func() error) error {
				old := available
				n := len(writes)
				err := fn()
				if stage == "release" {
					err = errPort
				}
				if err != nil {
					available = old
					writes = writes[:n]
				}
				return err
			}
			base.load = func(_ context.Context, tx *wager.Transaction) (wager.AccountingFacts, error) {
				g := pathWallet(t).Snapshot()
				g.Balance = pathMoney(t, available)
				op := g
				op.ID = "op"
				op.Balance = pathMoney(t, 1000)
				ref := win
				bid := "original-bet"
				if tx.ID() != root.ID() {
					ref = bet
					bid = "other-bet"
				}
				f := wager.AccountingFacts{Guarantee: g, Operational: op, Reference: &ref, BetID: bid, BetStatus: "OPEN", CommitmentID: "c", Remaining: 100, OriginalJournalID: "j"}
				if stage == "root rejected" && tx.ID() == root.ID() && available > 0 {
					f.AlreadyReversed = true
				}
				return f, nil
			}
			base.apply = func(_ context.Context, tx *wager.Transaction, _ wager.AccountingFacts, d wager.AccountingDecision, _ time.Time) error {
				sequence = append(sequence, "apply "+tx.ID())
				if stage == "root apply" && tx.ID() == root.ID() {
					return errPort
				}
				available = d.Guarantee.Balance.Minor()
				writes = append(writes, tx.ID())
				return nil
			}
			unit := liquidityTxStub{rollbackTxStub: base, liquidity: func(_ context.Context, w, b string, n int64, at time.Time) (port.RollbackLiquidityPlan, error) {
				if w != root.WalletID() || b != "original-bet" || n != 100 || at != pathTime {
					t.Fatal(w, b, n, at)
				}
				p := port.RollbackLiquidityPlan{Bets: []port.RecoverableBet{{Transaction: bet, ClosesAt: pathTime.Add(time.Minute)}}}
				switch stage {
				case "expired then eligible":
					expired := bet
					expired.ID = "expired"
					expired.ExternalID = "expired-external"
					p.Bets = append([]port.RecoverableBet{{Transaction: expired, ClosesAt: pathTime}}, p.Bets...)
				case "accumulate existing balance":
					p.Balance = 40
				case "available":
					available = 100
					p.Balance = 100
				case "waiting":
					p.Bets = nil
					p.AwaitResult = true
				case "window elapsed":
					p.Bets[0].ClosesAt = pathTime
				case "insufficient":
					p.Bets = nil
				case "lookup":
					return p, errPort
				}
				return p, nil
			}}
			err := pathSubmit(nil).process(context.Background(), unit, root)
			switch stage {
			case "recover", "available", "expired then eligible", "accumulate existing balance":
				if err != nil || root.Status() != wager.StatusProcessed || available != 0 {
					t.Fatal(root.Snapshot(), available, err)
				}
				want := []string{"child", "apply generated", "apply tx"}
				if stage == "available" {
					want = []string{"apply tx"}
				}
				if !reflect.DeepEqual(sequence, want) {
					t.Fatal(sequence)
				}
			case "waiting", "window elapsed":
				if err != nil || root.Status() != wager.StatusPendingRollback || root.Snapshot().ExpiresAt != nil || available != 0 {
					t.Fatal(root.Snapshot(), err)
				}
				if !reflect.DeepEqual(writes, []string{"PENDING_ROLLBACK", event.TypeWagerTransactionPendingRollback}) {
					t.Fatal(writes)
				}
			case "insufficient", "root rejected":
				code := wager.FailReversalInsufficientFunds
				if stage == "root rejected" {
					code = wager.FailAlreadyReversed
				}
				if err != nil || root.FailureCode() != code || available != 0 || !reflect.DeepEqual(writes, []string{"REJECTED", event.TypeWagerTransactionRejected}) {
					t.Fatal(root.Snapshot(), writes, err)
				}
			default:
				if !errors.Is(err, errPort) || available != 0 || len(writes) != 0 || !reflect.DeepEqual(root.Snapshot(), before) {
					t.Fatal(root.Snapshot(), writes, err)
				}
			}
		})
	}
}

func TestPendingRollbackSchedulingFailuresAndRepeat(t *testing.T) {
	for _, stage := range []string{"reference", "clock", "update", "event", "repeat"} {
		t.Run(stage, func(t *testing.T) {
			tx := pathTransaction(t, wager.KindRollback)
			uc := pathSubmit(nil)
			updates, events := 0, 0
			unit := txStub{t: transactionStub{update: func(context.Context, *wager.Transaction) error {
				updates++
				if stage == "update" {
					return errPort
				}
				return nil
			}}, o: outboxStub{add: func(context.Context, event.Outgoing) error {
				events++
				if stage == "event" {
					return errPort
				}
				return nil
			}}}
			ref := "original"
			if stage == "reference" {
				ref = tx.ID()
			}
			if stage == "clock" {
				uc.clock = clockFunc(func() time.Time { return pathTime.Add(-time.Second) })
			}
			err := uc.pendingRollback(context.Background(), unit, tx, ref)
			if stage == "repeat" {
				if err != nil {
					t.Fatal(err)
				}
				err = uc.pendingRollback(context.Background(), unit, tx, ref)
				if err != nil || updates != 2 || events != 1 {
					t.Fatal(updates, events, err)
				}
			} else if err == nil {
				t.Fatal("failure ignored")
			}
		})
	}
}

func TestSharedSettlementLiquidityRecoveryValidatesEveryWallet(t *testing.T) {
	for _, stage := range []string{"funded", "recover", "aggregate", "overflow", "lock", "lookup"} {
		t.Run(stage, func(t *testing.T) {
			facts := auditReversalFacts(t)
			if stage != "funded" {
				for id, a := range facts.Accounts {
					a.Balance = pathMoney(t, 0)
					facts.Accounts[id] = a
				}
			}
			if stage == "aggregate" || stage == "overflow" {
				facts.Payments[1].WalletID = facts.Payments[0].WalletID
			}
			if stage == "overflow" {
				facts.Payments[0].Amount = pathMoney(t, math.MaxInt64)
			}
			calls := 0
			unit := liquidityTxStub{rollbackTxStub: rollbackTxStub{audit: auditStub{lock: func(context.Context, string) (port.SettlementRecord, settlement.ReversalFacts, error) {
				if stage == "lock" {
					return port.SettlementRecord{}, facts, errPort
				}
				return port.SettlementRecord{BetID: "b"}, facts, nil
			}}}, liquidity: func(_ context.Context, w, b string, n int64, _ time.Time) (port.RollbackLiquidityPlan, error) {
				calls++
				if b != "b" || w == "" {
					t.Fatal(w, b)
				}
				if stage == "aggregate" && n != 200 {
					t.Fatal(n)
				}
				if stage == "lookup" {
					return port.RollbackLiquidityPlan{}, errPort
				}
				return port.RollbackLiquidityPlan{Balance: 1000}, nil
			}}
			err := pathSubmit(nil).recoverSettlementLiquidity(context.Background(), unit, "settlement", "root")
			switch stage {
			case "funded":
				if err != nil || calls != 0 {
					t.Fatal(calls, err)
				}
			case "recover":
				if err != nil || calls != 2 {
					t.Fatal(calls, err)
				}
			case "aggregate":
				if err != nil || calls != 1 {
					t.Fatal(calls, err)
				}
			default:
				if err == nil {
					t.Fatal("ignored failure")
				}
			}
		})
	}
	noAudit := struct {
		port.Tx
		port.RollbackLiquidity
	}{}
	if err := pathSubmit(nil).recoverSettlementLiquidity(context.Background(), noAudit, "s", "r"); err == nil {
		t.Fatal("missing audit store accepted")
	}
}

func TestUnwindPropagatesLiquidityWaitToOriginalOnly(t *testing.T) {
	for _, stage := range []string{"child waits", "root waits", "settlement load"} {
		t.Run(stage, func(t *testing.T) {
			root := pathTransaction(t, wager.KindRollback)
			win := pathTransaction(t, wager.KindWin).Snapshot()
			win.ID = "original"
			win.Status = wager.StatusProcessed
			var writes []string
			base := rollbackTxStub{}
			base.Tx = txStub{t: transactionStub{insert: func(_ context.Context, tx *wager.Transaction) error { writes = append(writes, tx.ID()); return nil }, update: func(_ context.Context, tx *wager.Transaction) error {
				writes = append(writes, tx.ID()+":"+string(tx.Status()))
				return nil
			}}, o: outboxStub{add: func(_ context.Context, e event.Outgoing) error { writes = append(writes, e.Type()); return nil }}}
			base.lock = func(context.Context, *wager.Transaction) (port.RollbackContext, error) {
				return port.RollbackContext{}, nil
			}
			base.scope = func(_ context.Context, fn func() error) error {
				n := len(writes)
				err := fn()
				if err != nil {
					writes = writes[:n]
				}
				return err
			}
			base.payments = func(context.Context, string) ([]wager.Snapshot, error) {
				if stage == "child waits" {
					return []wager.Snapshot{win}, nil
				}
				return nil, nil
			}
			base.load = func(context.Context, *wager.Transaction) (wager.AccountingFacts, error) {
				g := pathWallet(t).Snapshot()
				g.Balance = pathMoney(t, 0)
				op := g
				op.ID = "op"
				return wager.AccountingFacts{Guarantee: g, Operational: op, Reference: &win, BetID: "bet"}, nil
			}
			base.audit = auditStub{lock: func(context.Context, string) (port.SettlementRecord, settlement.ReversalFacts, error) {
				return port.SettlementRecord{}, settlement.ReversalFacts{}, errPort
			}}
			unit := liquidityTxStub{rollbackTxStub: base, liquidity: func(context.Context, string, string, int64, time.Time) (port.RollbackLiquidityPlan, error) {
				return port.RollbackLiquidityPlan{AwaitResult: true}, nil
			}}
			dependency := port.RollbackContext{HasPayments: true}
			if stage == "settlement load" {
				dependency = port.RollbackContext{SettlementID: "s", SettlementStatus: "PROCESSED"}
			}
			err := pathSubmit(nil).unwindRollback(context.Background(), unit, root, dependency, "original")
			if stage == "settlement load" {
				if !errors.Is(err, errPort) || len(writes) != 0 {
					t.Fatal(err, writes)
				}
			} else if err != nil || root.Status() != wager.StatusPendingRollback || !reflect.DeepEqual(writes, []string{"tx:PENDING_ROLLBACK", event.TypeWagerTransactionPendingRollback}) {
				t.Fatal(root.Snapshot(), writes, err)
			}
		})
	}
}
