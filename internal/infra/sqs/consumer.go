package sqs

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/alexandre/wagering/internal/platform/failpoint"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
)

// Consumer long-polls the FIFO queue and acks (deletes) a message ONLY after the handler committed durably.
type clientAPI interface {
	ReceiveMessage(context.Context, *awssqs.ReceiveMessageInput, ...func(*awssqs.Options)) (*awssqs.ReceiveMessageOutput, error)
	DeleteMessage(context.Context, *awssqs.DeleteMessageInput, ...func(*awssqs.Options)) (*awssqs.DeleteMessageOutput, error)
	ChangeMessageVisibility(context.Context, *awssqs.ChangeMessageVisibilityInput, ...func(*awssqs.Options)) (*awssqs.ChangeMessageVisibilityOutput, error)
}
type Consumer struct {
	client      clientAPI
	queueURL    string
	handler     port.MessageHandler
	log         *slog.Logger
	metrics     port.Metrics
	waitSeconds int32
	workTimeout time.Duration
}

func NewConsumer(c clientAPI, queueURL string, h port.MessageHandler, log *slog.Logger, m port.Metrics) *Consumer {
	return &Consumer{client: c, queueURL: queueURL, handler: h, log: log, metrics: m, waitSeconds: 20, workTimeout: 10 * time.Second}
}

// Run stops fetching when ctx is cancelled (SIGTERM); the message in flight is finished first.
func (c *Consumer) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		// Drain an already-issued long poll instead of abandoning its response.
		// A cancelled receive can still hide a message at the broker until visibility expires.
		receiveCtx, receiveCancel := context.WithTimeout(context.WithoutCancel(ctx), time.Duration(c.waitSeconds+1)*time.Second)
		out, err := c.client.ReceiveMessage(receiveCtx, &awssqs.ReceiveMessageInput{
			QueueUrl:                    &c.queueURL,
			MaxNumberOfMessages:         1,
			WaitTimeSeconds:             c.waitSeconds,
			MessageSystemAttributeNames: []types.MessageSystemAttributeName{types.MessageSystemAttributeNameApproximateReceiveCount},
		})
		receiveCancel()
		if err != nil {
			if ctx.Err() != nil {
				// Cancellation is a successful worker shutdown, not a receive failure.
				return nil //nolint:nilerr // Receive may fail while the owner requests shutdown.
			}
			c.log.Error("sqs receive failed", "err", err)
			c.metrics.Retry("sqs-receive")
			retry := time.NewTimer(2 * time.Second)
			select {
			case <-ctx.Done():
			case <-retry.C:
			}
			retry.Stop()
			continue
		}
		for _, m := range out.Messages {
			c.process(ctx, m)
		}
	}
	return nil
}

func (c *Consumer) process(ctx context.Context, m types.Message) {
	// Only identifiers are extracted; never log the raw envelope or financial values.
	var ids struct {
		MessageID string `json:"messageId"`
		Data      struct {
			WalletID   string `json:"walletId"`
			ProviderID string `json:"providerId"`
		} `json:"data"`
	}
	_ = json.Unmarshal([]byte(aws.ToString(m.Body)), &ids)
	scoped := *c
	scoped.log = c.log.With("messageId", ids.MessageID, "correlationId", ids.MessageID, "brokerMessageId", aws.ToString(m.MessageId), "walletId", ids.Data.WalletID, "providerId", ids.Data.ProviderID)
	c = &scoped
	// in-flight work is allowed to finish even after shutdown started
	hctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.workTimeout)
	defer cancel()

	if ctx.Err() != nil {
		c.setVisibility(hctx, m, 0)
		return
	}
	err := c.handler.Handle(hctx, []byte(aws.ToString(m.Body)))
	if err == nil {
		if failpoint.Enabled {
			failpoint.Hit("after_commit_before_ack")
		}
		c.ack(hctx, m)
	} else if errors.Is(err, apperr.ErrInvalidMessage) || errors.Is(err, apperr.ErrInvalidInput) || errors.Is(err, apperr.ErrIdempotencyConflict) || errors.Is(err, apperr.ErrMessageConflict) {
		// Broker redrive moves poison messages after 5 receives; never acknowledge uncommitted input.
		n, _ := strconv.Atoi(m.Attributes[string(types.MessageSystemAttributeNameApproximateReceiveCount)])
		if n >= 5 {
			c.metrics.DLQ()
		}
		c.log.Error("invalid message", "failureCode", "INVALID_MESSAGE")
		c.setVisibility(hctx, m, 2)
	} else {
		if ctx.Err() != nil {
			c.setVisibility(hctx, m, 0)
			return
		}
		c.metrics.Retry("sqs-consumer")
		c.log.Warn("transient failure, backing off", "err", err)
		n, _ := strconv.Atoi(m.Attributes[string(types.MessageSystemAttributeNameApproximateReceiveCount)])
		one := 2
		c.setVisibility(hctx, m, int32(min(one<<min(n, 9), 900)))
	}
}

func (c *Consumer) ack(ctx context.Context, m types.Message) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if _, err := c.client.DeleteMessage(ctx, &awssqs.DeleteMessageInput{QueueUrl: &c.queueURL, ReceiptHandle: m.ReceiptHandle}); err != nil {
		c.log.Error("sqs delete failed (message will be redelivered; inbox dedupes)", "err", err)
	}
}

func (c *Consumer) setVisibility(ctx context.Context, m types.Message, seconds int32) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	_, err := c.client.ChangeMessageVisibility(ctx, &awssqs.ChangeMessageVisibilityInput{
		QueueUrl: &c.queueURL, ReceiptHandle: m.ReceiptHandle, VisibilityTimeout: seconds,
	})
	if err != nil {
		c.log.Error("sqs change visibility failed", "err", err)
	}
}
