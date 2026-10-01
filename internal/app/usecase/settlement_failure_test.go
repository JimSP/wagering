package usecase

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/settlement"
)

type settlementTxStub struct {
	port.Tx
	store port.SettlementStore
}

func (s settlementTxStub) Settlements() port.SettlementStore { return s.store }

type settlementStoreStub struct {
	port.SettlementStore
	create      func(context.Context, settlement.Bet) error
	lock        func(context.Context, string) (settlement.Bet, error)
	find        func(context.Context, string) (port.SettlementRecord, error)
	commitments func(context.Context, string) ([]settlement.Commitment, error)
	bind        func(context.Context, string, string) error
}

func (s settlementStoreStub) CreateBet(c context.Context, b settlement.Bet) error {
	return s.create(c, b)
}

func (s settlementStoreStub) LockBet(c context.Context, id string) (settlement.Bet, error) {
	return s.lock(c, id)
}

func (s settlementStoreStub) FindSettlement(c context.Context, id string) (port.SettlementRecord, error) {
	return s.find(c, id)
}

func (s settlementStoreStub) Commitments(c context.Context, id string) ([]settlement.Commitment, error) {
	return s.commitments(c, id)
}

func (s settlementStoreStub) BindBet(c context.Context, tid, bid string) error {
	return s.bind(c, tid, bid)
}

func TestCreateBetValidatesBeforePersistenceAndNormalizesIdentity(t *testing.T) {
	for _, stage := range []string{"success", "invalid id", "missing provider", "missing round", "missing game", "invalid currency", "missing storage", "create failure"} {
		t.Run(stage, func(t *testing.T) {
			b := settlement.Bet{ID: strings.ToUpper(settlementTestID), ProviderID: "provider", RoundID: "round", GameID: "game", Currency: "BRL", Status: "CLOSED"}
			calls := 0
			var saved settlement.Bet
			u := &uowStub{tx: settlementTxStub{store: settlementStoreStub{create: func(_ context.Context, b settlement.Bet) error {
				calls++
				saved = b
				if stage == "create failure" {
					return errPort
				}
				return nil
			}}}}
			var want error
			switch stage {
			case "invalid id":
				b.ID = "bad"
				want = apperr.ErrInvalidInput
			case "missing provider":
				b.ProviderID = " "
				want = apperr.ErrInvalidInput
			case "missing round":
				b.RoundID = " "
				want = apperr.ErrInvalidInput
			case "missing game":
				b.GameID = " "
				want = apperr.ErrInvalidInput
			case "invalid currency":
				b.Currency = "invalid"
				want = apperr.ErrInvalidInput
			case "missing storage":
				u.tx = txStub{}
			case "create failure":
				want = errPort
			}
			err := pathSubmit(u).Settlements().Create(context.Background(), b)
			if stage == "missing storage" {
				if err == nil || !strings.Contains(err.Error(), "persistence is not configured") {
					t.Fatal(err)
				}
			} else if !errors.Is(err, want) {
				t.Fatal(err)
			}
			if stage == "success" || stage == "create failure" {
				b.ID = settlementTestID
				b.Status = "OPEN"
				b.CreatedAt = pathTime
				b.BettingWindowSeconds = 300
				if calls != 1 || saved != b {
					t.Fatal(saved, calls)
				}
			} else if calls != 0 {
				t.Fatal("invalid bet reached storage")
			}
			if errors.Is(want, apperr.ErrInvalidInput) && u.writes != 0 {
				t.Fatal("invalid input opened transaction")
			}
		})
	}
}

func TestConfirmSettlementStopsBeforeSavingIncompleteFacts(t *testing.T) {
	for _, stage := range []string{"invalid id", "empty result", "nonpositive allocation", "nonpositive return", "missing storage", "lock failure", "lookup failure", "commitment failure"} {
		t.Run(stage, func(t *testing.T) {
			id := settlementTestID
			d := settlement.Distribution{ResultID: "result", Returns: []settlement.Return{{ExternalID: "bet", Money: pathMoney(t, 100)}}}
			var calls []string
			store := settlementStoreStub{
				lock: func(_ context.Context, id string) (settlement.Bet, error) {
					calls = append(calls, "lock")
					if id != settlementTestID {
						t.Fatal(id)
					}
					if stage == "lock failure" {
						return settlement.Bet{}, errPort
					}
					return settlement.Bet{ID: id, Status: "OPEN", Currency: "BRL"}, nil
				},
				find: func(_ context.Context, id string) (port.SettlementRecord, error) {
					calls = append(calls, "find")
					if id != settlementTestID {
						t.Fatal(id)
					}
					if stage == "lookup failure" {
						return port.SettlementRecord{}, errPort
					}
					return port.SettlementRecord{}, apperr.ErrNotFound
				},
				commitments: func(_ context.Context, id string) ([]settlement.Commitment, error) {
					calls = append(calls, "commitments")
					if id != settlementTestID {
						t.Fatal(id)
					}
					return nil, errPort
				},
			}
			u := &uowStub{tx: settlementTxStub{store: store}}
			want := apperr.ErrInvalidInput
			var wantCalls []string
			switch stage {
			case "invalid id":
				id = "bad"
			case "empty result":
				d.ResultID = " "
			case "nonpositive allocation":
				d.Allocations = []settlement.Transfer{{From: "a", To: "b", Money: pathMoney(t, 0)}}
			case "nonpositive return":
				d.Returns[0].Money = pathMoney(t, 0)
			case "missing storage":
				u.tx = txStub{}
			case "lock failure":
				want = errPort
				wantCalls = []string{"lock"}
			case "lookup failure":
				want = errPort
				wantCalls = []string{"lock", "find"}
			case "commitment failure":
				want = errPort
				wantCalls = []string{"lock", "find", "commitments"}
			}
			out, err := pathSubmit(u).Settlements().Confirm(context.Background(), id, d)
			if stage == "missing storage" {
				if err == nil || !strings.Contains(err.Error(), "persistence is not configured") {
					t.Fatal(err)
				}
			} else if !errors.Is(err, want) {
				t.Fatal(err)
			}
			if out != (port.SettlementRecord{}) || !reflect.DeepEqual(calls, wantCalls) {
				t.Fatal(out, calls, wantCalls)
			}
			if stage != "missing storage" && errors.Is(want, apperr.ErrInvalidInput) && u.writes != 0 {
				t.Fatal("invalid request opened transaction")
			}
		})
	}
}
