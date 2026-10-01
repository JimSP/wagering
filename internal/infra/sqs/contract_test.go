package sqs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type contractClient struct {
	receive    func(context.Context, *sdk.ReceiveMessageInput) (*sdk.ReceiveMessageOutput, error)
	delete     func(context.Context, *sdk.DeleteMessageInput) error
	visibility func(context.Context, *sdk.ChangeMessageVisibilityInput) error
}

func (c contractClient) ReceiveMessage(ctx context.Context, in *sdk.ReceiveMessageInput, _ ...func(*sdk.Options)) (*sdk.ReceiveMessageOutput, error) {
	return c.receive(ctx, in)
}

func (c contractClient) DeleteMessage(ctx context.Context, in *sdk.DeleteMessageInput, _ ...func(*sdk.Options)) (*sdk.DeleteMessageOutput, error) {
	return &sdk.DeleteMessageOutput{}, c.delete(ctx, in)
}

func (c contractClient) ChangeMessageVisibility(ctx context.Context, in *sdk.ChangeMessageVisibilityInput, _ ...func(*sdk.Options)) (*sdk.ChangeMessageVisibilityOutput, error) {
	return &sdk.ChangeMessageVisibilityOutput{}, c.visibility(ctx, in)
}

func checkBudget(t *testing.T, ctx context.Context, want time.Duration) {
	t.Helper()
	deadline, ok := ctx.Deadline()
	remaining := time.Until(deadline)
	if !ok || remaining > want || remaining < want-time.Second {
		t.Fatalf("remaining=%v want approximately=%v", remaining, want)
	}
}

func TestConsumerPollAndHandlerHaveIndependentBudgets(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	cl := contractClient{receive: func(c context.Context, in *sdk.ReceiveMessageInput) (*sdk.ReceiveMessageOutput, error) {
		calls++
		checkBudget(t, c, 21*time.Second)
		if in.WaitTimeSeconds != 20 || in.MaxNumberOfMessages != 1 || aws.ToString(in.QueueUrl) != "queue" {
			t.Fatal(in)
		}
		cancel()
		if c.Err() != nil {
			t.Fatal("poll cancelled during drain")
		}
		return &sdk.ReceiveMessageOutput{}, nil
	}}
	c := NewConsumer(cl, "queue", nil, slog.Default(), metrics.New())
	if err := c.Run(ctx); err != nil || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	called := 0
	cl.delete = func(c context.Context, in *sdk.DeleteMessageInput) error {
		checkBudget(t, c, 2*time.Second)
		return nil
	}
	c.client = cl
	c.handler = handlerFunc(func(ctx context.Context, b []byte) error {
		called++
		checkBudget(t, ctx, 10*time.Second)
		if string(b) != "body" {
			t.Fatal(string(b))
		}
		return nil
	})
	c.process(context.Background(), types.Message{Body: aws.String("body")})
	if called != 1 {
		t.Fatal(called)
	}
}

func TestTransientBackoffAndCleanupLogsMatchActualOutcome(t *testing.T) {
	for n, want := range []int32{2, 4, 8, 16, 32, 64, 128, 256, 512, 900, 900} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			cl := &fakeClient{visibility: -1}
			c := NewConsumer(cl, "queue", handlerFunc(func(context.Context, []byte) error { return errors.New("offline") }), slog.Default(), metrics.New())
			c.process(context.Background(), types.Message{Attributes: map[string]string{string(types.MessageSystemAttributeNameApproximateReceiveCount): fmt.Sprint(n)}})
			if cl.visibility != want || cl.ack {
				t.Fatalf("visibility=%d want=%d ack=%v", cl.visibility, want, cl.ack)
			}
		})
	}
	for _, failed := range []bool{false, true} {
		t.Run(fmt.Sprint("cleanup failure=", failed), func(t *testing.T) {
			var logs bytes.Buffer
			calls := 0
			cause := errors.New("offline")
			check := func(ctx context.Context, queue, receipt string) error {
				calls++
				checkBudget(t, ctx, 2*time.Second)
				if queue != "queue" || receipt != "receipt" {
					t.Fatal(queue, receipt)
				}
				if failed {
					return cause
				}
				return nil
			}
			cl := contractClient{delete: func(ctx context.Context, in *sdk.DeleteMessageInput) error {
				return check(ctx, aws.ToString(in.QueueUrl), aws.ToString(in.ReceiptHandle))
			}, visibility: func(ctx context.Context, in *sdk.ChangeMessageVisibilityInput) error {
				if in.VisibilityTimeout != 16 {
					t.Fatal(in.VisibilityTimeout)
				}
				return check(ctx, aws.ToString(in.QueueUrl), aws.ToString(in.ReceiptHandle))
			}}
			c := NewConsumer(cl, "queue", nil, slog.New(slog.NewJSONHandler(&logs, nil)), metrics.New())
			m := types.Message{ReceiptHandle: aws.String("receipt")}
			c.ack(context.Background(), m)
			c.setVisibility(context.Background(), m, 16)
			if calls != 2 || strings.Contains(logs.String(), "sqs delete failed") != failed || strings.Contains(logs.String(), "sqs change visibility failed") != failed {
				t.Fatalf("calls=%d logs=%s", calls, logs.String())
			}
		})
	}
}
