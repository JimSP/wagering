package wager

import (
	"errors"
	"fmt"
)

// FailureCode is a stable, documented code persisted in REJECTED/FAILED transactions.
type FailureCode string

const (
	FailResultAlreadyLost         FailureCode = "RESULT_ALREADY_LOST" // WIN contradicts a processed LOSS in the same context
	FailBetNotClosed              FailureCode = "BET_NOT_CLOSED"      // WIN received before the betting deadline
	FailBetClosed                 FailureCode = "BET_CLOSED"
	FailInvalidInput              FailureCode = "INVALID_INPUT"
	FailWalletNotFound            FailureCode = "WALLET_NOT_FOUND"
	FailCurrencyMismatch          FailureCode = "CURRENCY_MISMATCH"
	FailInsufficientFunds         FailureCode = "INSUFFICIENT_FUNDS"          // BET without balance
	FailReversalInsufficientFunds FailureCode = "REVERSAL_INSUFFICIENT_FUNDS" // ROLLBACK/REFUND needing more than available
	FailReferenceNotFound         FailureCode = "REFERENCE_NOT_FOUND"         // attempts/TTL exhausted
	FailReferenceNotProcessed     FailureCode = "REFERENCE_NOT_PROCESSED"     // reference exists but ended REJECTED/FAILED
	FailReferenceMismatch         FailureCode = "REFERENCE_MISMATCH"          // provider/player/wallet/currency/round differ
	FailReversalAmountMismatch    FailureCode = "REVERSAL_AMOUNT_MISMATCH"
	FailInvalidReferenceKind      FailureCode = "INVALID_REFERENCE_KIND"
	FailAlreadyReversed           FailureCode = "ALREADY_REVERSED"
	FailConflictingReversal       FailureCode = "CONFLICTING_REVERSAL" // historical rejection code; current duplicates use ALREADY_REVERSED
	FailInternalPermanent         FailureCode = "INTERNAL_PERMANENT_ERROR"
)

type DomainError struct {
	Code FailureCode
	Msg  string
}

func (e *DomainError) Error() string { return string(e.Code) + ": " + e.Msg }

// Is compares by code so wrapped errors match sentinels via errors.Is.
func (e *DomainError) Is(target error) bool {
	t, ok := target.(*DomainError)
	return ok && t.Code == e.Code
}

func newErr(c FailureCode, msg string) *DomainError { return &DomainError{c, msg} }

var (
	ErrInvalidInput           = newErr(FailInvalidInput, "invalid input")
	ErrInsufficientFunds      = newErr(FailInsufficientFunds, "insufficient funds")
	ErrReversalInsufficient   = newErr(FailReversalInsufficientFunds, "insufficient funds to reverse")
	ErrReferenceNotFound      = newErr(FailReferenceNotFound, "reference not found")
	ErrReferenceNotProcessed  = newErr(FailReferenceNotProcessed, "reference not processed")
	ErrReferenceMismatch      = newErr(FailReferenceMismatch, "reference mismatch")
	ErrReversalAmountMismatch = newErr(FailReversalAmountMismatch, "reversal amount mismatch")
	ErrInvalidReferenceKind   = newErr(FailInvalidReferenceKind, "invalid reference kind")
	ErrAlreadyReversed        = newErr(FailAlreadyReversed, "already reversed")
	ErrConflictingReversal    = newErr(FailConflictingReversal, "conflicting reversal")

	ErrTerminalState = errors.New("wager: transaction is in a terminal state")
)

// CodeOf extracts the failure code of a (possibly wrapped) DomainError.
func CodeOf(err error) (FailureCode, bool) {
	var d *DomainError
	if errors.As(err, &d) {
		return d.Code, true
	}
	return "", false
}

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, fmt.Sprintf(format, a...))
}
