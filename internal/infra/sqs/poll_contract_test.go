//go:build go1.25

package sqs

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type pollClient struct {
	fakeClient
	receive func(context.Context, *sdk.ReceiveMessageInput) (*sdk.ReceiveMessageOutput, error)
}

func (c *pollClient) ReceiveMessage(ctx context.Context, in *sdk.ReceiveMessageInput, _ ...func(*sdk.Options)) (*sdk.ReceiveMessageOutput, error) {
	return c.receive(ctx, in)
}

func TestPollRetryAndCancellation(t *testing.T) {
	for _, stop := range []string{"before", "during-error", "during-backoff", "after-backoff"} {
		t.Run(stop, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				calls := 0
				var logs bytes.Buffer
				rec := metrics.New()
				start := time.Now()
				cl := &pollClient{receive: func(c context.Context, in *sdk.ReceiveMessageInput) (*sdk.ReceiveMessageOutput, error) {
					calls++
					if calls == 2 && stop == "after-backoff" && time.Since(start) != 2*time.Second {
						t.Errorf("retry occurred after %v, want 2s", time.Since(start))
					}
					if calls > 2 {
						t.Fatal("poll repeated after cancellation")
					}
					if aws.ToString(in.QueueUrl) != "queue" || in.MaxNumberOfMessages != 1 {
						t.Fatal(in)
					}
					if stop == "during-error" || calls == 2 {
						cancel()
					}
					return nil, errors.New("poll failed")
				}}
				c := NewConsumer(cl, "queue", handlerFunc(func(context.Context, []byte) error { t.Error("handler called without message"); return nil }), slog.New(slog.NewJSONHandler(&logs, nil)), rec)
				if stop == "before" {
					cancel()
				}
				done := make(chan error, 1)
				go func() { done <- c.Run(ctx) }()
				synctest.Wait()
				if stop == "during-backoff" {
					cancel()
				}
				if stop == "after-backoff" {
					time.Sleep(2 * time.Second)
				}
				synctest.Wait()
				if e := <-done; e != nil {
					t.Fatal(e)
				}
				want := 1
				if stop == "before" {
					want = 0
				}
				if stop == "after-backoff" {
					want = 2
					if time.Since(start) != 2*time.Second {
						t.Fatal(time.Since(start))
					}
				}
				if calls != want {
					t.Fatal(calls, want)
				}
				if stop == "during-backoff" || stop == "after-backoff" {
					if !strings.Contains(logs.String(), "sqs receive failed") {
						t.Fatal(logs.String())
					}
					w := httptest.NewRecorder()
					rec.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
					if !strings.Contains(w.Body.String(), `wagering_retries_total{component="sqs-receive"} 1`) {
						t.Fatal(w.Body.String())
					}
				}
			})
		})
	}
}

func TestEachPoisonErrorAndRedriveThreshold(t *testing.T) {
	for _, cause := range []error{apperr.ErrInvalidMessage, apperr.ErrInvalidInput, apperr.ErrIdempotencyConflict, apperr.ErrMessageConflict} {
		for _, count := range []string{"4", "5", "6", "bad"} {
			cl := &fakeClient{visibility: -1}
			rec := metrics.New()
			var logs bytes.Buffer
			c := NewConsumer(cl, "queue", handlerFunc(func(context.Context, []byte) error { return cause }), slog.New(slog.NewJSONHandler(&logs, nil)), rec)
			c.process(context.Background(), types.Message{MessageId: aws.String("message"), Attributes: map[string]string{string(types.MessageSystemAttributeNameApproximateReceiveCount): count}})
			if cl.ack || cl.visibility != 2 || !strings.Contains(logs.String(), `"failureCode":"INVALID_MESSAGE"`) {
				t.Fatal(cl, logs.String())
			}
			w := httptest.NewRecorder()
			rec.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
			want := "0"
			if count == "5" || count == "6" {
				want = "1"
			}
			if !strings.Contains(w.Body.String(), "wagering_poison_redrive_threshold_total "+want) {
				t.Fatal(w.Body.String())
			}
		}
	}
}
