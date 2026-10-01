//go:build go1.25

package sqs

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/alexandre/wagering/internal/infra/config"
	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/alexandre/wagering/internal/platform/worker"
)

func TestConfiguredConsumerShutdownBudgets(t *testing.T) {
	for _, c := range []struct {
		shutdown time.Duration
		poll     int32
		work     time.Duration
	}{{8 * time.Second, 5, 3 * time.Second}, {10 * time.Second, 7, 5 * time.Second}, {25 * time.Second, 20, 10 * time.Second}} {
		got := configuredConsumer(nil, config.Config{ShutdownTimeout: c.shutdown}, nil, nil, nil)
		if got.waitSeconds != c.poll || got.workTimeout != c.work {
			t.Fatalf("shutdown=%v poll=%d work=%v", c.shutdown, got.waitSeconds, got.workTimeout)
		}
	}
}

func TestRoleAndDLQMonitorRegistration(t *testing.T) {
	for _, c := range []struct {
		role bool
		dlq  string
	}{{false, "dlq"}, {true, ""}, {true, "dlq"}} {
		synctest.Test(t, func(t *testing.T) {
			var logs bytes.Buffer
			log := slog.New(slog.NewJSONHandler(&logs, nil))
			rec := metrics.New()
			lc := &sqsLifecycle{}
			cfg := config.Config{WagerQueueURL: "wagers", EventsQueueURL: "events", WagerDLQURL: c.dlq, ShutdownTimeout: 25 * time.Second}
			if c.role {
				cfg.Roles = []string{"sqs-consumer"}
			}
			polls, depths := 0, 0
			start := time.Now()
			cl := sdkClient(func(r *http.Request) (*http.Response, error) {
				if strings.HasSuffix(r.Header.Get("X-Amz-Target"), "ReceiveMessage") {
					polls++
					<-r.Context().Done()
					return nil, r.Context().Err()
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body["QueueUrl"] != "wagers" && body["QueueUrl"] != "events" {
					depths++
					if time.Since(start) != 5*time.Second {
						t.Errorf("DLQ poll at %v, want 5s", time.Since(start))
					}
				}
				return sdkResponse(`{"Attributes":{"ApproximateNumberOfMessages":"3","ApproximateNumberOfMessagesNotVisible":"2"}}`), nil
			})
			group := worker.NewGroup(log, nil, cfg)
			register(lc, group, cfg, cl, handlerFunc(func(context.Context, []byte) error { t.Error("unexpected message"); return nil }), log, rec, rec, nil)
			group.Install(lc)
			for _, h := range lc.hooks {
				if h.OnStart != nil {
					if err := h.OnStart(context.Background()); err != nil {
						t.Fatal(err)
					}
				}
			}
			time.Sleep(5 * time.Second)
			synctest.Wait()
			wantPolls, wantDepths := 0, 0
			if c.role {
				wantPolls = 1
				if c.dlq != "" {
					wantDepths = 1
				}
			}
			if polls != wantPolls || depths != wantDepths {
				t.Errorf("role=%v dlq=%q polls=%d depths=%d", c.role, c.dlq, polls, depths)
			}
			if strings.Contains(logs.String(), "DLQ metrics unavailable") {
				t.Error(logs.String())
			}
			for i := len(lc.hooks) - 1; i >= 0; i-- {
				if lc.hooks[i].OnStop != nil {
					if err := lc.hooks[i].OnStop(context.Background()); err != nil {
						t.Error(err)
					}
				}
			}
		})
	}
}
