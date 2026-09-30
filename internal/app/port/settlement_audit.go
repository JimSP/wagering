package port

import (
	"context"
	"time"

	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/settlement"
)

type SettlementPosting struct {
	ID            string      `json:"entryId"`
	JournalID     string      `json:"journalId"`
	TransactionID string      `json:"transactionId"`
	AccountID     string      `json:"accountId"`
	WalletID      string      `json:"walletId"`
	Role          string      `json:"role"`
	Direction     string      `json:"direction"`
	Money         money.Money `json:"money"`
	Before        money.Money `json:"balanceBefore"`
	After         money.Money `json:"balanceAfter"`
	Version       int64       `json:"accountVersion"`
	Sequence      int64       `json:"sequence"`
	Reverses      string      `json:"reversesJournalId,omitempty"`
	CreatedAt     time.Time   `json:"createdAt"`
}
type SettlementPayment struct {
	ID          string      `json:"transactionId"`
	WalletID    string      `json:"walletId"`
	ReferenceID string      `json:"betTransactionId"`
	Status      string      `json:"status"`
	Money       money.Money `json:"money"`
}
type SettlementAudit struct {
	SettlementRecord
	Payments []SettlementPayment `json:"payments"`
	Postings []SettlementPosting `json:"postings"`
}
type SettlementAuditStore interface {
	AuditSettlement(context.Context, string) (SettlementAudit, error)
	LockReversal(context.Context, string) (SettlementRecord, settlement.ReversalFacts, error)
	ReverseSettlement(context.Context, string, time.Time) error
}
type SettlementAuditTransaction interface{ SettlementAudit() SettlementAuditStore }
