package sqs

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/infra/config"
)

// Publisher sends outbox events to wager-events.fifo.
// MessageGroupId = aggregateId (ordering per aggregate); MessageDeduplicationId = eventId (republication-safe).
type Publisher struct {
	client             *awssqs.Client
	queueURL           string
	settlementQueueURL string
}

func NewPublisher(c *awssqs.Client, cfg config.Config) *Publisher {
	return &Publisher{client: c, queueURL: cfg.EventsQueueURL, settlementQueueURL: cfg.SettlementQueueURL}
}

func (p *Publisher) Publish(ctx context.Context, ev event.Outgoing) error {
	queueURL, body := p.queueURL, string(ev.Payload())
	if ev.Type() == "SettlementRequested" {
		if p.settlementQueueURL == "" {
			return errors.New("private settlement queue is not configured")
		}
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(ev.Payload(), &envelope); err != nil {
			return err
		}
		payload, err := json.Marshal(struct {
			MessageID  string          `json:"messageId"`
			Type       string          `json:"type"`
			OccurredAt any             `json:"occurredAt"`
			Data       json.RawMessage `json:"data"`
		}{ev.EventID(), ev.Type(), ev.OccurredAt(), envelope.Data})
		if err != nil {
			return err
		}
		queueURL = p.settlementQueueURL
		body = string(payload)
	}
	_, err := p.client.SendMessage(ctx, &awssqs.SendMessageInput{
		QueueUrl:               &queueURL,
		MessageBody:            aws.String(body),
		MessageGroupId:         aws.String(ev.AggregateID()),
		MessageDeduplicationId: aws.String(ev.EventID()),
		MessageAttributes: map[string]types.MessageAttributeValue{
			"eventType": {DataType: aws.String("String"), StringValue: aws.String(ev.Type())},
		},
	})
	return apperr.Transient(err)
}
