package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

func TestProcessRollbackLiquidityRequiresRecoverableFailure(t *testing.T) {
	for _, stage := range []string{"invalid reference", "already reversed", "counterparty insufficient at equal guarantee"} {
		t.Run(stage, func(t *testing.T) {
			root := pathTransaction(t, wager.KindRollback)
			reference := pathTransaction(t, wager.KindWin).Snapshot()
			reference.ID = "original"
			reference.Status = wager.StatusProcessed
			guarantee := pathWallet(t).Snapshot()
			guarantee.Balance = pathMoney(t, 0)
			operational := guarantee
			operational.ID = "operational"
			operational.Balance = pathMoney(t, 1000)
			facts := wager.AccountingFacts{Guarantee: guarantee, Operational: operational, Reference: &reference, BetID: "bet"}
			want := wager.FailReferenceMismatch
			switch stage {
			case "invalid reference":
				reference.ProviderID = "different-provider"
			case "already reversed":
				facts.AlreadyReversed = true
				want = wager.FailAlreadyReversed
			case "counterparty insufficient at equal guarantee":
				facts.Guarantee.Balance = root.Amount()
				counterparty := operational
				counterparty.ID = "counterparty"
				counterparty.Balance = pathMoney(t, 99)
				facts.ReferenceSettlementID = "settlement"
				facts.ReversalAccounts = map[string]wallet.Snapshot{facts.Guarantee.ID: facts.Guarantee, operational.ID: operational, counterparty.ID: counterparty}
				facts.ReversalJournals = []wager.OriginalJournal{{DebitAccountID: operational.ID, CreditAccountID: counterparty.ID, Amount: root.Amount()}}
				want = wager.FailReversalInsufficientFunds
			}
			updates, events := 0, 0
			unit := liquidityTxStub{rollbackTxStub: rollbackTxStub{
				Tx: txStub{t: transactionStub{update: func(_ context.Context, tx *wager.Transaction) error {
					updates++
					if tx.Status() != wager.StatusRejected || tx.FailureCode() != want {
						t.Fatalf("persisted unexpected rejection: %+v", tx.Snapshot())
					}
					return nil
				}}, o: outboxStub{add: func(_ context.Context, e event.Outgoing) error {
					events++
					if e.Type() != event.TypeWagerTransactionRejected {
						t.Fatalf("unexpected event %s", e.Type())
					}
					return nil
				}}},
				lock: func(context.Context, *wager.Transaction) (port.RollbackContext, error) {
					return port.RollbackContext{}, nil
				},
				load: func(context.Context, *wager.Transaction) (wager.AccountingFacts, error) { return facts, nil },
				scope: func(context.Context, func() error) error {
					t.Fatal("rejection must not start recovery scope")
					return nil
				},
				apply: func(context.Context, *wager.Transaction, wager.AccountingFacts, wager.AccountingDecision, time.Time) error {
					t.Fatal("rejection must not apply accounting")
					return nil
				},
			}, liquidity: func(context.Context, string, string, int64, time.Time) (port.RollbackLiquidityPlan, error) {
				t.Fatal("unrecoverable rejection must not request liquidity")
				return port.RollbackLiquidityPlan{}, nil
			}}
			if err := pathSubmit(nil).process(context.Background(), unit, root); err != nil {
				t.Fatal(err)
			}
			if root.Status() != wager.StatusRejected || root.FailureCode() != want || updates != 1 || events != 1 {
				t.Fatalf("rejection=%+v updates=%d events=%d", root.Snapshot(), updates, events)
			}
		})
	}
}
