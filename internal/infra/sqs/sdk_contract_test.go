package sqs

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/alexandre/wagering/internal/infra/config"
	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/alexandre/wagering/internal/platform/worker"
	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/sqs"
	"go.uber.org/fx"
)

type sdkHTTP func(*http.Request) (*http.Response, error)

func (f sdkHTTP) Do(r *http.Request) (*http.Response, error) { return f(r) }
func sdkClient(f sdkHTTP) *sdk.Client {
	return sdk.New(sdk.Options{Region: "us-east-1", BaseEndpoint: aws.String("https://sqs.example.test"), Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "test", SecretAccessKey: "test"}, nil
	}), HTTPClient: f, RetryMaxAttempts: 1})
}

func sdkResponse(body string) *http.Response {
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/x-amz-json-1.0"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func TestSDKPublisherAndReadinessContracts(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ev, e := event.NewProcessed(event.Meta{EventID: "event", AggregateID: "wallet", CorrelationID: "corr", OccurredAt: at}, event.ProcessedData{TransactionID: "tx", Kind: "OPENING", WalletID: "wallet", PlayerID: "player", Money: event.MoneyDTO{Amount: "1.00", Currency: "BRL"}}).ToOutgoing()
	if e != nil {
		t.Fatal(e)
	}
	calls := 0
	cause := errors.New("network offline")
	fail := false
	cl := sdkClient(func(r *http.Request) (*http.Response, error) {
		calls++
		if fail {
			return nil, cause
		}
		var body map[string]any
		if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
			t.Fatal(e)
		}
		if strings.HasSuffix(r.Header.Get("X-Amz-Target"), "SendMessage") {
			if body["QueueUrl"] != "https://sqs.example.test/events.fifo" || body["MessageBody"] != string(ev.Payload()) || body["MessageGroupId"] != "wallet" || body["MessageDeduplicationId"] != "event" {
				t.Fatal(body)
			}
			attrs := body["MessageAttributes"].(map[string]any)["eventType"].(map[string]any)
			if attrs["StringValue"] != ev.Type() || attrs["DataType"] != "String" {
				t.Fatal(attrs)
			}
		} else {
			if body["QueueUrl"] != "https://sqs.example.test/wagers.fifo" {
				t.Fatal(body)
			}
			deadline, ok := r.Context().Deadline()
			if !ok || time.Until(deadline) > 2*time.Second || time.Until(deadline) < time.Second {
				t.Fatal("readiness deadline")
			}
		}
		return sdkResponse(`{}`), nil
	})
	cfg := config.Config{WagerQueueURL: "https://sqs.example.test/wagers.fifo", EventsQueueURL: "https://sqs.example.test/events.fifo"}
	p := NewPublisher(cl, cfg)
	if e = p.Publish(context.Background(), ev); e != nil {
		t.Fatal(e)
	}
	r := NewReadiness(cl, cfg)
	if r.Name() != "sqs" {
		t.Fatal(r.Name())
	}
	if e = r.Check(context.Background()); e != nil {
		t.Fatal(e)
	}
	fail = true
	if e = p.Publish(context.Background(), ev); !apperr.IsTransient(e) || !errors.Is(e, cause) {
		t.Fatal(e)
	}
	if e = r.Check(context.Background()); !errors.Is(e, cause) {
		t.Fatal(e)
	}
	if calls != 4 {
		t.Fatal(calls)
	}
}

type sqsLifecycle struct{ hooks []fx.Hook }

func (l *sqsLifecycle) Append(h fx.Hook) { l.hooks = append(l.hooks, h) }
func TestQueueStartupAndDLQMetrics(t *testing.T) {
	for _, role := range []bool{false, true} {
		for _, dlq := range []string{"", "https://sqs.example.test/dlq.fifo"} {
			cfg := config.Config{WagerQueueURL: "wagers", EventsQueueURL: "events", WagerDLQURL: dlq, ShutdownTimeout: 25 * time.Second}
			if role {
				cfg.Roles = []string{"sqs-consumer"}
			}
			lc := &sqsLifecycle{}
			rec := metrics.New()
			calls := 0
			fail := false
			cause := errors.New("broker unavailable")
			cl := sdkClient(func(r *http.Request) (*http.Response, error) {
				calls++
				if fail {
					return nil, cause
				}
				var b map[string]any
				if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
					t.Error(err)
				}
				want := "wagers"
				if calls == 2 {
					want = "events"
				}
				if b["QueueUrl"] != want {
					t.Fatal(b)
				}
				return sdkResponse(`{}`), nil
			})
			register(lc, &worker.Group{}, cfg, cl, handlerFunc(func(context.Context, []byte) error { return nil }), slog.New(slog.NewTextHandler(io.Discard, nil)), rec, rec, nil)
			if len(lc.hooks) != 1 {
				t.Fatal(lc.hooks)
			}
			if e := lc.hooks[0].OnStart(context.Background()); e != nil || calls != 2 {
				t.Fatal(calls, e)
			}
			fail = true
			if e := lc.hooks[0].OnStart(context.Background()); !errors.Is(e, cause) {
				t.Fatal(e)
			}
		}
	}
	for _, body := range []string{`{"Attributes":{"ApproximateNumberOfMessages":"3","ApproximateNumberOfMessagesNotVisible":"2"}}`, `{"Attributes":{"ApproximateNumberOfMessages":"bad","ApproximateNumberOfMessagesNotVisible":"2"}}`, `{"Attributes":{"ApproximateNumberOfMessages":"3","ApproximateNumberOfMessagesNotVisible":"bad"}}`, "transport error"} {
		rec := metrics.New()
		cl := sdkClient(func(r *http.Request) (*http.Response, error) {
			if body == "transport error" {
				return nil, errors.New(body)
			}
			return sdkResponse(body), nil
		})
		e := updateDLQDepth(context.Background(), cl, "dlq", rec)
		if body == `{"Attributes":{"ApproximateNumberOfMessages":"3","ApproximateNumberOfMessagesNotVisible":"2"}}` {
			if e != nil {
				t.Fatal(e)
			}
			w := httptest.NewRecorder()
			rec.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
			for _, line := range []string{`wagering_dlq_messages{state="visible"} 3`, `wagering_dlq_messages{state="inflight"} 2`} {
				if !strings.Contains(w.Body.String(), line) {
					t.Fatal(w.Body.String())
				}
			}
		} else if e == nil {
			t.Fatal("error suppressed", body)
		}
	}
}

func TestAWSClientConfiguration(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	for _, endpoint := range []string{"", "https://sqs.example.test"} {
		cl, e := NewClient(config.Config{AWSRegion: "eu-west-1", AWSEndpointURL: endpoint})
		if e != nil {
			t.Fatal(e)
		}
		o := cl.Options()
		if o.Region != "eu-west-1" || aws.ToString(o.BaseEndpoint) != endpoint {
			t.Fatal(o.Region, o.BaseEndpoint)
		}
	}
	t.Setenv("AWS_CONFIG_FILE", t.TempDir()+"/missing")
	t.Setenv("AWS_PROFILE", "profile-does-not-exist")
	if cl, e := NewClient(config.Config{}); e == nil || cl != nil {
		t.Fatal(cl, e)
	}
}

func TestSettlementPublisherUsesPrivateQueueAndStableDelivery(t *testing.T) {
	at := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	ev, err := event.NewSettlementRequested(event.Meta{EventID: "delivery", AggregateID: "bet", CorrelationID: "settlement", OccurredAt: at}, event.SettlementRequestedData{SettlementID: "0192f28f-5dc0-7d58-bdb2-814ad6a0f4a7"}).ToOutgoing()
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	cl := sdkClient(func(r *http.Request) (*http.Response, error) {
		calls++
		var body struct {
			QueueURL               string `json:"QueueUrl"`
			MessageBody            string
			MessageDeduplicationID string `json:"MessageDeduplicationId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		var envelope struct {
			MessageID string            `json:"messageId"`
			Type      string            `json:"type"`
			Data      map[string]string `json:"data"`
		}
		if err := json.Unmarshal([]byte(body.MessageBody), &envelope); err != nil {
			t.Fatal(err)
		}
		if body.QueueURL != "https://sqs.example.test/settlements.fifo" || body.MessageDeduplicationID != "delivery" || envelope.MessageID != "delivery" || envelope.Type != "SettlementRequested" || len(envelope.Data) != 1 || envelope.Data["settlementId"] != "0192f28f-5dc0-7d58-bdb2-814ad6a0f4a7" {
			t.Fatalf("body=%+v envelope=%+v", body, envelope)
		}
		return sdkResponse(`{"MessageId":"id"}`), nil
	})
	p := NewPublisher(cl, config.Config{EventsQueueURL: "https://sqs.example.test/events.fifo", SettlementQueueURL: "https://sqs.example.test/settlements.fifo"})
	for i := 0; i < 2; i++ {
		if err := p.Publish(context.Background(), ev); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 2 {
		t.Fatal(calls)
	}
	missing := NewPublisher(cl, config.Config{EventsQueueURL: "https://sqs.example.test/events.fifo"})
	if err := missing.Publish(context.Background(), ev); err == nil || calls != 2 {
		t.Fatal("settlement leaked to ordinary events queue", err)
	}
}

func TestQueueStartupSkipsEmptyURLAndChecksFollowingQueues(t *testing.T) {
	lc := &sqsLifecycle{}
	rec := metrics.New()
	var urls []string
	cl := sdkClient(func(r *http.Request) (*http.Response, error) {
		var body struct {
			QueueURL string `json:"QueueUrl"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		urls = append(urls, body.QueueURL)
		return sdkResponse(`{}`), nil
	})
	register(lc, &worker.Group{}, config.Config{EventsQueueURL: "events", SettlementQueueURL: "settlements"}, cl, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), rec, rec, nil)
	if err := lc.hooks[0].OnStart(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(urls) != 2 || urls[0] != "events" || urls[1] != "settlements" {
		t.Fatalf("checked queues: %v", urls)
	}
}
