package sqs

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// Exercises the production consumer and SDK call boundary. Handler completion
// is controlled here; real SQL commit, inbox and broker durability require the
// integration suite. In particular, an error never proves the DB rolled back.
func TestConsumerDeleteRequiresSuccessfulHandlerCompletion(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want []string
	}{
		{"success", nil, []string{"handler-enter", "handler-success", "delete"}},
		{"transient-error", apperr.Transient(errors.New("connection lost")), []string{"handler-enter", "handler-error", "visibility"}},
		{"unknown-commit-outcome", errors.New("commit response lost"), []string{"handler-enter", "handler-error", "visibility"}},
		{"invalid-message", apperr.ErrInvalidMessage, []string{"handler-enter", "handler-error", "visibility"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var observed []string
			client := contractClient{
				delete: func(_ context.Context, in *sdk.DeleteMessageInput) error {
					if aws.ToString(in.QueueUrl) != "queue" || aws.ToString(in.ReceiptHandle) != "receipt" {
						t.Fatal("delete used another queue or receipt", in)
					}
					observed = append(observed, "delete")
					return nil
				},
				visibility: func(_ context.Context, in *sdk.ChangeMessageVisibilityInput) error {
					if aws.ToString(in.QueueUrl) != "queue" || aws.ToString(in.ReceiptHandle) != "receipt" {
						t.Fatal("retry used another queue or receipt", in)
					}
					observed = append(observed, "visibility")
					return nil
				},
			}
			handler := handlerFunc(func(_ context.Context, body []byte) error {
				observed = append(observed, "handler-enter")
				if string(body) != "message-body" {
					t.Fatal("consumer changed the message body")
				}
				if tc.err != nil {
					observed = append(observed, "handler-error")
					return tc.err
				}
				observed = append(observed, "handler-success")
				return nil
			})
			consumer := NewConsumer(client, "queue", handler, slog.New(slog.NewTextHandler(io.Discard, nil)), metrics.New())
			consumer.process(context.Background(), types.Message{Body: aws.String("message-body"), ReceiptHandle: aws.String("receipt")})
			if !reflect.DeepEqual(observed, tc.want) {
				t.Fatalf("consumer order=%v; want %v", observed, tc.want)
			}
		})
	}
}
