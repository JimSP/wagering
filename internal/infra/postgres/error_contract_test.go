package postgres

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"

	"github.com/alexandre/wagering/internal/app/apperr"
)

func TestTransportFailuresPreserveCauseAndRetryClassification(t *testing.T) {
	for _, cause := range []error{io.EOF, io.ErrUnexpectedEOF, &net.DNSError{Err: "offline", Name: "db"}, context.DeadlineExceeded, apperr.ErrConcurrentModification} {
		t.Run(cause.Error(), func(t *testing.T) {
			got := classify(fmt.Errorf("query: %w", cause))
			if !apperr.IsTransient(got) || apperr.IsPermanent(got) || !errors.Is(got, cause) {
				t.Fatalf("classification=%v", got)
			}
		})
	}
	cause := errors.New("unclassified")
	if got := classify(cause); !errors.Is(got, cause) || apperr.IsTransient(got) {
		t.Fatal(got)
	}
}
