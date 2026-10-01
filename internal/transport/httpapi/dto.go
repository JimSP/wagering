package httpapi

import (
	"fmt"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/domain/money"
)

// ---- Contracts (see api/openapi.yaml) -------------------------------------

type MoneyDTO struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

func toMoneyDTO(m money.Money) MoneyDTO { return MoneyDTO{m.Amount(), m.Currency()} }

func toMoneyDTOPtr(m *money.Money) *MoneyDTO {
	if m == nil {
		return nil
	}
	d := toMoneyDTO(*m)
	return &d
}

func (d MoneyDTO) parse() (money.Money, error) {
	m, err := money.Parse(d.Amount, d.Currency)
	if err != nil {
		return money.Money{}, fmt.Errorf("%w: %w", apperr.ErrInvalidInput, err)
	}
	return m, nil
}

type OpenWalletRequest struct {
	PlayerID       string   `json:"playerId"`
	InitialBalance MoneyDTO `json:"initialBalance"`
}

type WalletResponse struct {
	ID       string   `json:"id"`
	PlayerID string   `json:"playerId"`
	Balance  MoneyDTO `json:"balance"`
	Version  int64    `json:"version"`
}

type SubmitTransactionRequest struct {
	BetID                          string   `json:"betId,omitempty"`
	ProviderID                     string   `json:"providerId"`
	ExternalTransactionID          string   `json:"externalTransactionId"`
	PlayerID                       string   `json:"playerId"`
	WalletID                       string   `json:"walletId"`
	RoundID                        string   `json:"roundId"`
	GameID                         string   `json:"gameId"`
	Kind                           string   `json:"kind"`
	Money                          MoneyDTO `json:"money"`
	ReferenceExternalTransactionID string   `json:"referenceExternalTransactionId,omitempty"`
}

type SubmitTransactionResponse struct {
	TransactionID    string    `json:"transactionId"`
	Status           string    `json:"status"`
	Balance          *MoneyDTO `json:"balance,omitempty"`
	FailureCode      string    `json:"failureCode,omitempty"`
	IdempotentReplay bool      `json:"idempotentReplay"`
}

type TransactionResponse struct {
	TransactionID         string     `json:"transactionId"`
	ProviderID            string     `json:"providerId,omitempty"`
	ExternalTransactionID string     `json:"externalTransactionId,omitempty"`
	WalletID              string     `json:"walletId"`
	Kind                  string     `json:"kind"`
	Status                string     `json:"status"`
	Money                 MoneyDTO   `json:"money"`
	Balance               *MoneyDTO  `json:"balance,omitempty"`
	FailureCode           string     `json:"failureCode,omitempty"`
	NextAttemptAt         *time.Time `json:"nextAttemptAt,omitempty"`
}

type LedgerEntryDTO struct {
	WalletID      string    `json:"walletId"`
	ID            string    `json:"id"`
	TransactionID string    `json:"transactionId"`
	Direction     string    `json:"direction"`
	Money         MoneyDTO  `json:"money"`
	BalanceBefore MoneyDTO  `json:"balanceBefore"`
	BalanceAfter  MoneyDTO  `json:"balanceAfter"`
	CreatedAt     time.Time `json:"createdAt"`
}

type LedgerResponse struct {
	Entries    []LedgerEntryDTO `json:"entries"`
	NextCursor string           `json:"nextCursor,omitempty"`
}

type ReconciliationResponse struct {
	WalletID          string   `json:"walletId"`
	StoredBalance     MoneyDTO `json:"storedBalance"`
	CalculatedBalance MoneyDTO `json:"calculatedBalance"`
	Difference        MoneyDTO `json:"difference"`
	Consistent        bool     `json:"consistent"`
	CheckedEntries    int64    `json:"checkedEntries"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}
