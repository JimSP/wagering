package postgres

import (
	"errors"
	"testing"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestSQLConflictsAreRetryableAndObservable(t *testing.T) {
	for _, code := range []string{"40001", "40P01", "55P03"} {
		err := classify(&pgconn.PgError{Code: code})
		if !apperr.IsTransient(err) || !errors.Is(err, apperr.ErrConcurrentModification) {
			t.Fatalf("%s: %v", code, err)
		}
	}
	err := classify(&pgconn.PgError{Code: "08006"})
	if !apperr.IsTransient(err) || errors.Is(err, apperr.ErrConcurrentModification) {
		t.Fatal(err)
	}
}
