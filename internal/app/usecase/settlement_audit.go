package usecase

import (
	"context"
	"errors"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/settlement"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/google/uuid"
)

func auditStore(tx port.Tx) (port.SettlementAuditStore, error) {
	s, ok := tx.(port.SettlementAuditTransaction)
	if !ok {
		return nil, errors.New("settlement audit storage missing")
	}
	return s.SettlementAudit(), nil
}

func (u *Settlements) Get(ctx context.Context, id string) (port.SettlementAudit, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return port.SettlementAudit{}, apperr.Invalid("invalid settlementId")
	}
	var out port.SettlementAudit
	err = u.uow.DoSnapshot(ctx, func(ctx context.Context, tx port.Tx) error {
		r, err := auditStore(tx)
		if err != nil {
			return err
		}
		out, err = r.AuditSettlement(ctx, parsed.String())
		return err
	})
	if err != nil {
		return port.SettlementAudit{}, err
	}
	return out, nil
}

// Reverse compensates the complete settlement. Its ID is also the replay key:
// there can be only one full inverse of a settled result.
func (u *Settlements) Reverse(ctx context.Context, id string) (port.SettlementRecord, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return port.SettlementRecord{}, apperr.Invalid("invalid settlementId")
	}
	var out port.SettlementRecord
	err = u.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		var e error
		out, e = u.reverseInTx(ctx, tx, parsed.String(), "")
		return e
	})
	if err != nil {
		return port.SettlementRecord{}, err
	}
	return out, nil
}

func (u *Settlements) reverseInTx(ctx context.Context, tx port.Tx, id, cause string) (port.SettlementRecord, error) {
	r, err := auditStore(tx)
	if err != nil {
		return port.SettlementRecord{}, err
	}
	record, facts, err := r.LockReversal(ctx, id)
	if err != nil {
		return port.SettlementRecord{}, err
	}
	if record.Status == "REVERSED" {
		return record, nil
	}
	if err = settlement.ValidateReversal(facts, u.clock.Now()); err != nil {
		return port.SettlementRecord{}, err
	}
	if cause == "" {
		cause = record.ID
	}
	for _, payment := range facts.Payments {
		// The settlement lock/status owns replay. A fresh opaque operation ID
		// avoids collisions with provider-supplied external transaction identities.
		reversalID := u.ids.NewID()
		external := "settlement-reversal:" + reversalID
		hash := wager.PayloadHash(wager.HashInput{ProviderID: payment.ProviderID, ExternalTransactionID: external, PlayerID: payment.PlayerID, WalletID: payment.WalletID, RoundID: payment.RoundID, GameID: payment.GameID, Kind: "ROLLBACK", Amount: payment.Amount.Amount(), Currency: payment.Amount.Currency(), ReferenceExternalTransactionID: payment.ExternalID})
		reversal, err := wager.NewExternal(wager.ExternalParams{ID: reversalID, ProviderID: payment.ProviderID, ExternalID: external, IdempotencyKey: external, PayloadHash: hash, WalletID: payment.WalletID, PlayerID: payment.PlayerID, RoundID: payment.RoundID, GameID: payment.GameID, Kind: wager.KindRollback, Amount: payment.Amount, ReferenceExternalID: payment.ExternalID, CorrelationID: cause}, u.clock.Now())
		if err != nil {
			return port.SettlementRecord{}, err
		}
		if err = reversal.ResolveReference(payment.ID); err != nil {
			return port.SettlementRecord{}, err
		}
		if err = tx.Transactions().Insert(ctx, reversal); err != nil {
			return port.SettlementRecord{}, err
		}
	}
	if err = r.ReverseSettlement(ctx, record.ID, u.clock.Now()); err != nil {
		return port.SettlementRecord{}, err
	}
	record.Status = "REVERSED"
	return record, nil
}
