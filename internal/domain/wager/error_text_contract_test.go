package wager_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/domain/wager"
)

func TestPendingRetryErrorsHaveExactText(t *testing.T) {
	tx := external(t, wager.KindRefund)
	base := tx.Snapshot()
	expiry := base.CreatedAt.Add(time.Hour)
	for _, c := range []struct {
		name     string
		attempts int
		expires  *time.Time
		want     string
	}{
		{"fresh", 0, nil, ""},
		{"attempt only", 1, nil, "INVALID_INPUT: invalid input: retry history on initial pending state"},
		{"expiry only", 0, &expiry, "INVALID_INPUT: invalid input: retry history on initial pending state"},
		{"both", 1, &expiry, "INVALID_INPUT: invalid input: retry history on initial pending state"},
		{"negative", -1, nil, "INVALID_INPUT: invalid input: invalid snapshot"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := base
			s.Attempts = c.attempts
			s.ExpiresAt = c.expires
			got, err := wager.Rehydrate(s)
			if c.want == "" {
				if err != nil || got == nil || !reflect.DeepEqual(got.Snapshot(), s) {
					t.Fatal(got, err)
				}
			} else {
				if got != nil || !errors.Is(err, wager.ErrInvalidInput) || err.Error() != c.want {
					t.Fatalf("got=%v err=%v want=%q", got, err, c.want)
				}
			}
			if !reflect.DeepEqual(tx.Snapshot(), base) {
				t.Fatal("input changed")
			}
		})
	}
}
