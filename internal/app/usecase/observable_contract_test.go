package usecase_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/usecase"
)

func TestSubmitRejectsEachMissingInboxFieldBeforeAnyEffect(t *testing.T) {
	for _, field := range []string{"id", "hash"} {
		t.Run(field, func(t *testing.T) {
			s := scenarioWith(t, 10000)
			in := s.input("bet", "BET", "1.00", "")
			in.Source = usecase.SourceSQS
			in.InboxMessageID = "message"
			in.InboxHash = "hash"
			if field == "id" {
				in.InboxMessageID = ""
			} else {
				in.InboxHash = ""
			}
			before := s.m.s.copy()
			calls := s.m.calls
			got, err := s.submit.Execute(context.Background(), in)
			if !errors.Is(err, apperr.ErrInvalidInput) || !reflect.DeepEqual(got, usecase.SubmitResult{}) || s.m.calls != calls || !reflect.DeepEqual(before, s.m.s) {
				t.Fatalf("result=%+v err=%v effects=%d", got, err, s.m.calls-calls)
			}
		})
	}
}

func TestSubmissionObservabilityFollowsCommittedOutcome(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "success and replay", true: "failure"}[failed], func(t *testing.T) {
			s := scenarioWith(t, 10000)
			in := s.input("bet", "BET", "1.00", "")
			s.metrics.results = nil
			s.metrics.duplicates = nil

			var logs bytes.Buffer
			old := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
			defer slog.SetDefault(old)
			cause := errors.New("outbox unavailable")
			before := s.m.s.copy()
			if failed {
				s.m.failEvent = cause
			}
			result, err := s.submit.Execute(context.Background(), in)
			if failed {
				if !errors.Is(err, cause) || !reflect.DeepEqual(result, usecase.SubmitResult{}) || !reflect.DeepEqual(before, s.m.s) || len(s.metrics.results) != 0 || len(s.metrics.duplicates) != 0 || !strings.Contains(logs.String(), `"msg":"transaction handling failed"`) || strings.Contains(logs.String(), `"msg":"transaction handled"`) {
					t.Fatalf("err=%v metrics=%v logs=%s", err, s.metrics.results, logs.String())
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			replay, err := s.submit.Execute(context.Background(), in)
			if err != nil || !replay.IdempotentReplay || !reflect.DeepEqual(s.metrics.results, []string{"BET:PROCESSED", "BET:PROCESSED"}) || !reflect.DeepEqual(s.metrics.duplicates, []string{"HTTP"}) || strings.Count(logs.String(), `"msg":"transaction handled"`) != 2 || strings.Contains(logs.String(), `"msg":"transaction handling failed"`) {
				t.Fatalf("replay=%+v err=%v results=%v duplicates=%v logs=%s", replay, err, s.metrics.results, s.metrics.duplicates, logs.String())
			}
		})
	}
}
