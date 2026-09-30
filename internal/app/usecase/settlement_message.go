package usecase

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/google/uuid"
)

// ConsumeSettlementMessage is wired only to the private settlement queue.
// Wager ingress cannot select this handler by changing the envelope type.
type ConsumeSettlementMessage struct{ submit *SubmitTransaction }

func NewConsumeSettlementMessage(s *SubmitTransaction) *ConsumeSettlementMessage {
	return &ConsumeSettlementMessage{s}
}

func (u *ConsumeSettlementMessage) Handle(ctx context.Context, body []byte) error {
	var msg struct {
		MessageID  string    `json:"messageId"`
		Type       string    `json:"type"`
		OccurredAt time.Time `json:"occurredAt"`
		Data       struct {
			SettlementID string `json:"settlementId"`
		} `json:"data"`
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if dec.Decode(&msg) != nil || dec.Decode(new(any)) != io.EOF || msg.MessageID == "" || msg.Type != "SettlementRequested" || msg.OccurredAt.IsZero() {
		return apperr.ErrInvalidMessage
	}
	id, err := uuid.Parse(msg.Data.SettlementID)
	if err != nil {
		return apperr.ErrInvalidMessage
	}
	sum := sha256.Sum256(body)
	return u.submit.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		executor, ok := tx.(port.SettlementDeliveryTransaction)
		if !ok {
			return apperr.Permanent(apperr.Invalid("settlement executor not configured"))
		}
		return executor.SettleDelivery(ctx, msg.MessageID, hex.EncodeToString(sum[:]), id.String(), u.submit.clock.Now())
	})
}
