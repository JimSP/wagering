//go:build integration

package postgres

import (
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/event"
	"github.com/google/uuid"
)

type openingPrecisionClock struct{ now time.Time }

func (c openingPrecisionClock) Now() time.Time { return c.now }

func TestOpeningWithSubmicrosecondClockPersistsAndRehydratesOutbox(t *testing.T) {
	p, ctx := accountingDB(t)
	for _, nanos := range []int{123456100, 123456500, 123456999} {
		t.Run(time.Duration(nanos).String(), func(t *testing.T) {
			clock := openingPrecisionClock{time.Date(2026, 9, 30, 12, 0, 0, nanos, time.UTC)}
			wallet, err := usecase.NewOpenWallet(NewUnitOfWork(p), clock, settlementTestIDs{}).Execute(ctx, usecase.OpenWalletInput{PlayerID: uuid.NewString(), InitialBalance: repoMoney(t, 10000)})
			if err != nil {
				t.Fatal(err)
			}
			accountingBalances(t, p, ctx, wallet.ID, 10000, 0)
			rows, err := p.Query(ctx, `SELECT event_id::text,aggregate_id::text,event_type,payload,occurred_at FROM outbox_events WHERE aggregate_id=$1`, wallet.ID)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			count := 0
			for rows.Next() {
				var id, aggregate, kind string
				var payload []byte
				var occurred time.Time
				if err = rows.Scan(&id, &aggregate, &kind, &payload, &occurred); err != nil {
					t.Fatal(err)
				}
				if _, err = event.RehydrateOutgoing(id, aggregate, kind, payload, occurred, 0); err != nil {
					t.Fatal(err)
				}
				count++
			}
			if err = rows.Err(); err != nil {
				t.Fatal(err)
			}
			if count != 2 {
				t.Fatalf("opening events: %d", count)
			}
		})
	}
}
