package wager

type Kind string

const (
	KindOpening  Kind = "OPENING"
	KindBet      Kind = "BET"
	KindWin      Kind = "WIN"
	KindLoss     Kind = "LOSS"
	KindRefund   Kind = "REFUND"
	KindRollback Kind = "ROLLBACK"
)

// ParseExternalKind accepts only kinds allowed via HTTP/SQS (OPENING is internal-only).
func ParseExternalKind(s string) (Kind, error) {
	switch k := Kind(s); k {
	case KindBet, KindWin, KindLoss, KindRefund, KindRollback:
		return k, nil
	case KindOpening:
		return "", invalid("OPENING is reserved for internal wallet opening")
	default:
		return "", invalid("unknown kind %q", s)
	}
}

func (k Kind) IsReversal() bool { return k == KindRefund || k == KindRollback }

type Status string

const (
	StatusPending          Status = "PENDING"
	StatusPendingReference Status = "PENDING_REFERENCE"
	StatusPendingRollback  Status = "PENDING_ROLLBACK"
	StatusProcessed        Status = "PROCESSED"
	StatusRejected         Status = "REJECTED"
	StatusFailed           Status = "FAILED"
)

func (s Status) IsTerminal() bool {
	return s == StatusProcessed || s == StatusRejected || s == StatusFailed
}

func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusPendingReference, StatusPendingRollback, StatusProcessed, StatusRejected, StatusFailed:
		return true
	}
	return false
}

type Origin string

const (
	OriginInternal Origin = "INTERNAL"
	OriginExternal Origin = "EXTERNAL"
)

type Direction string

const (
	Debit  Direction = "DEBIT"
	Credit Direction = "CREDIT"
)
