package sqs

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type fakeClient struct {
	clientAPI
	visibility int32
	ack        bool
}

func (f *fakeClient) ChangeMessageVisibility(ctx context.Context, in *sdk.ChangeMessageVisibilityInput, _ ...func(*sdk.Options)) (*sdk.ChangeMessageVisibilityOutput, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	f.visibility = in.VisibilityTimeout
	return &sdk.ChangeMessageVisibilityOutput{}, nil
}

func (f *fakeClient) DeleteMessage(ctx context.Context, _ *sdk.DeleteMessageInput, _ ...func(*sdk.Options)) (*sdk.DeleteMessageOutput, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	f.ack = true
	return &sdk.DeleteMessageOutput{}, nil
}

type handlerFunc func(context.Context, []byte) error

func (f handlerFunc) Handle(c context.Context, b []byte) error { return f(c, b) }
func TestCleanupHasIndependentDeadlineAndLogsOnlyIdentifiers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &fakeClient{visibility: -1}
	var logs bytes.Buffer
	c := NewConsumer(client, "queue", handlerFunc(func(context.Context, []byte) error { return errors.New("dependency unavailable") }), slog.New(slog.NewJSONHandler(&logs, nil)), metrics.New())
	m := types.Message{MessageId: aws.String("broker"), Body: aws.String(`{"messageId":"message","data":{"walletId":"wallet","providerId":"provider","money":{"amount":"SECRET_AMOUNT"}}}`)}
	c.setVisibility(ctx, m, 0)
	c.ack(ctx, m)
	if client.visibility != 0 || !client.ack {
		t.Fatal("cleanup used cancelled context")
	}
	c.process(context.Background(), m)
	for _, want := range []string{`"messageId":"message"`, `"walletId":"wallet"`, `"providerId":"provider"`, `"correlationId":"message"`} {
		if !strings.Contains(logs.String(), want) {
			t.Fatal(logs.String())
		}
	}
	if strings.Contains(logs.String(), "SECRET_AMOUNT") {
		t.Fatal("financial payload logged")
	}
}

type drainingClient struct {
	fakeClient
	entered, release chan struct{}
}

func (f *drainingClient) ReceiveMessage(ctx context.Context, _ *sdk.ReceiveMessageInput, _ ...func(*sdk.Options)) (*sdk.ReceiveMessageOutput, error) {
	close(f.entered)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-f.release:
		return &sdk.ReceiveMessageOutput{Messages: []types.Message{{MessageId: aws.String("last-poll")}}}, nil
	}
}

func TestShutdownDrainsIssuedPollAndReleasesWithoutHandling(t *testing.T) {
	cl := &drainingClient{fakeClient: fakeClient{visibility: -1}, entered: make(chan struct{}), release: make(chan struct{})}
	var logs bytes.Buffer
	c := NewConsumer(cl, "queue", handlerFunc(func(context.Context, []byte) error { t.Error("handled after shutdown"); return nil }), slog.New(slog.NewJSONHandler(&logs, nil)), metrics.New())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	defer cancel()
	select {
	case <-cl.entered:
	case err := <-done:
		t.Fatalf("consumer returned before issuing a poll: %v", err)
	case <-time.After(time.Second):
		t.Fatal("consumer did not issue a poll")
	}
	cancel()
	close(cl.release)
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("poll not drained")
	}
	if cl.visibility != 0 || cl.ack {
		t.Fatal("message was not released")
	}
}
