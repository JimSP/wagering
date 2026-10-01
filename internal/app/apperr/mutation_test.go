package apperr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/alexandre/wagering/internal/app/apperr"
)

type classifiedCause struct{ operation string }

func (e *classifiedCause) Error() string { return e.operation + " unavailable" }

func TestClassificationPreservesCauseAndRetryContract(t *testing.T) {
	cause := &classifiedCause{operation: "database"}
	for _, tc := range []struct {
		name             string
		wrap             func(error) error
		retry, permanent bool
	}{
		{"transient", apperr.Transient, true, false},
		{"permanent", apperr.Permanent, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.wrap(nil); got != nil {
				t.Fatalf("nil cause became an error: %v", got)
			}
			classified := tc.wrap(cause)
			if got, want := classified.Error(), tc.name+": database unavailable"; got != want {
				t.Fatalf("message=%q, want %q", got, want)
			}
			if errors.Unwrap(classified) != cause { //nolint:errorlint // Verify immediate cause identity, not transitive matching.
				t.Fatal("wrapper lost original cause identity")
			}
			for _, err := range []error{classified, fmt.Errorf("apply transaction: %w", classified), errors.Join(errors.New("secondary failure"), classified)} {
				if !errors.Is(err, cause) {
					t.Fatalf("cause lost through wrapping: %v", err)
				}
				var recovered *classifiedCause
				if !errors.As(err, &recovered) || recovered != cause {
					t.Fatalf("typed cause lost: %v", err)
				}
				if apperr.IsTransient(err) != tc.retry || apperr.IsPermanent(err) != tc.permanent {
					t.Fatalf("classification changed: %v", err)
				}
			}
		})
	}
	for _, err := range []error{nil, cause, apperr.ErrInvalidInput, fmt.Errorf("operation: %w", apperr.ErrNotFound)} {
		if apperr.IsTransient(err) || apperr.IsPermanent(err) {
			t.Fatalf("unclassified error was assigned infrastructure policy: %v", err)
		}
	}
}

func TestInvalidPreservesSentinelAndFormattedObservation(t *testing.T) {
	err := apperr.Invalid("wallet %s: limit %d exceeds %d", "wallet-7", 21, 20)
	if got, want := err.Error(), "invalid input: wallet wallet-7: limit 21 exceeds 20"; got != want {
		t.Fatalf("message=%q, want %q", got, want)
	}
	if errors.Unwrap(err) != apperr.ErrInvalidInput || !errors.Is(fmt.Errorf("request: %w", err), apperr.ErrInvalidInput) { //nolint:errorlint // Verify the immediate sentinel as well as transitive matching.
		t.Fatal("validation sentinel identity lost")
	}
	if errors.Is(err, errors.New("invalid input")) || errors.Is(err, apperr.ErrInvalidMessage) || apperr.IsTransient(err) || apperr.IsPermanent(err) {
		t.Fatal("validation failure misclassified")
	}
	if got := apperr.Invalid("missing kind").Error(); got != "invalid input: missing kind" {
		t.Fatal(got)
	}
}
