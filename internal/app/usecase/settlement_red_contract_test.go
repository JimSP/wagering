package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/wallet"
)

// Existing real use cases execute over the existing persistence fake. No deposit,
// guarantee repository or settlement algorithm is implemented by this fixture.
// It deliberately seeds corrupted storage, impossible under the SQL pair constraint.
// Rehydration must fail without inventing a guarantee or authorizing spending.
func revisedLegacyScenario(t *testing.T, balance int64) *scenario {
	t.Helper()
	s := &scenario{t: t, m: &memory{s: newState()}, c: &clock{at}, id: &ids{}, metrics: &observation{}}
	w, err := wallet.New(s.id.NewID(), player, moneyOf(t, balance), at)
	if err != nil {
		t.Fatal(err)
	}
	s.wallet = w.ID()
	s.m.s.wallets[w.ID()] = w.Snapshot()
	s.submit = usecase.NewSubmitTransaction(s.m, s.c, s.id, s.metrics)
	return s
}

func revisedInput(s *scenario, source usecase.Source, external, kind, amount string) usecase.SubmitInput {
	in := s.input(external, kind, amount, "")
	in.Source = source
	if source == usecase.SourceSQS {
		in.InboxMessageID = "msg:" + external
		in.InboxHash = "hash:" + external
	}
	return in
}

// This fixture has no guarantee binding at all. Its rejection must diagnose
// that structural defect, not prohibit the WIN operation or require a reference.
func TestRevisedStandaloneWINCannotCreateUnfundedMoney(t *testing.T) {
	for _, source := range []usecase.Source{usecase.SourceHTTP, usecase.SourceSQS} {
		for _, balance := range []int64{0, 2500, 10000} {
			t.Run(fmt.Sprintf("%s/balance-%d", source, balance), func(t *testing.T) {
				s := revisedLegacyScenario(t, balance)
				before := s.m.s.copy()
				in := revisedInput(s, source, "unfunded-win", "WIN", "35.00")
				result, err := s.submit.Execute(context.Background(), in)
				if !errors.Is(err, wallet.ErrInvalidWallet) {
					t.Errorf("standalone WIN: err=%v result=%+v; want corrupted account snapshot", err, result)
				}
				if !reflect.DeepEqual(result, usecase.SubmitResult{}) {
					t.Errorf("invalid request returned financial result: %+v", result)
				}
				if !reflect.DeepEqual(before, s.m.s) {
					t.Errorf("unfunded WIN changed durable state: balance %d -> %d; ledger %d -> %d; events %d -> %d; inbox %d -> %d", balance, s.m.s.wallets[s.wallet].Balance.Minor(), len(before.ledger), len(s.m.s.ledger), len(before.events), len(s.m.s.events), len(before.inbox), len(s.m.s.inbox))
				}
			})
		}
	}
}

// Missing guarantee is not the same as an existing guarantee with zero balance.
// Neither legacy wallet cash nor another player's cash supplies an own guarantee.
func TestRevisedBETCannotSpendWithoutItsOwnGuarantee(t *testing.T) {
	for _, source := range []usecase.Source{usecase.SourceHTTP, usecase.SourceSQS} {
		t.Run(string(source), func(t *testing.T) {
			s := revisedLegacyScenario(t, 10000)
			other, err := wallet.New(s.id.NewID(), "0192f28f-5dc0-7d58-bdb2-814ad6a0f4a2", moneyOf(t, 100000), at)
			if err != nil {
				t.Fatal(err)
			}
			s.m.s.wallets[other.ID()] = other.Snapshot()
			before := s.m.s.copy()
			result, err := s.submit.Execute(context.Background(), revisedInput(s, source, "bet-no-guarantee", "BET", "25.00"))
			if !errors.Is(err, wallet.ErrInvalidWallet) {
				t.Errorf("missing own guarantee: err=%v result=%+v; want corrupted account snapshot", err, result)
			}
			if !reflect.DeepEqual(result, usecase.SubmitResult{}) {
				t.Errorf("missing guarantee returned financial result: %+v", result)
			}
			if !reflect.DeepEqual(before, s.m.s) {
				t.Error("missing guarantee consumed identity or changed persisted state")
			}
			if !reflect.DeepEqual(before.wallets, s.m.s.wallets) {
				t.Error("missing own guarantee changed an account balance/version")
			}
			if !reflect.DeepEqual(before.ledger, s.m.s.ledger) {
				t.Error("missing own guarantee created financial postings")
			}
			for _, e := range s.m.s.events[len(before.events):] {
				if e.Type() == "WalletBalanceChanged" || e.Type() == "WagerTransactionProcessed" {
					t.Errorf("missing guarantee emitted success event: %s", e.Type())
				}
			}
		})
	}
}

// Funded positive controls cannot pass by rejecting every command.
func TestRevisedCommittedMovementCannotHaveOnlyOneSide(t *testing.T) {
	p := pairedScenarioWith(t, 10000, 0)
	before := p.m.s.copy()
	r := p.send(pairInput(p, usecase.SourceHTTP, "movement", "25.00"))
	assertPairedBET(t, p, before, r, pairedBETCases()[0].want)
}

func TestRevisedBETMustNotEmitWalletDebitAsStakeCommitment(t *testing.T) {
	tc := pairedBETCases()[4]
	p := pairedScenarioWith(t, tc.guarantee, tc.operational)
	before := p.m.s.copy()
	r := p.send(pairInput(p, usecase.SourceHTTP, "bet-direction", tc.amount))
	assertPairedBET(t, p, before, r, tc.want)
}

// Existing non-negativity is retained. The unfunded 25 -> payout 35 example
// must fail without changing the account; a losing commitment supplies the 10
// through the future settlement use case, not by relaxing Debit.
func TestRevisedPayoutCannotOverdrawOperationalWallet(t *testing.T) {
	s := revisedLegacyScenario(t, 2500)
	w, err := wallet.Rehydrate(s.m.s.wallets[s.wallet])
	if err != nil {
		t.Fatal(err)
	}
	before := w.Snapshot()
	_, _, err = w.Debit(moneyOf(t, 3500), at)
	if !errors.Is(err, wallet.ErrInsufficientFunds) || err.Error() != "wallet: insufficient funds" {
		t.Fatalf("err=%v; want wallet: insufficient funds", err)
	}
	if !reflect.DeepEqual(before, w.Snapshot()) {
		t.Fatal("refused payout changed wallet state")
	}
}
