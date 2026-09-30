//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/settlement"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fifoIDs struct{ first string }

func (g *fifoIDs) NewID() string {
	if g.first != "" {
		id := g.first
		g.first = ""
		return id
	}
	return uuid.NewString()
}

func fifoSubmit(t *testing.T, p *pgxpool.Pool, ctx context.Context, in usecase.SubmitInput, at time.Time, id string) usecase.SubmitResult {
	t.Helper()
	r, err := usecase.NewSubmitTransaction(NewUnitOfWork(p), accountingTestClock{at}, &fifoIDs{first: id}, metrics.New()).Execute(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func fifoReference(t *testing.T, p *pgxpool.Pool, ctx context.Context, got usecase.SubmitResult, want string) {
	t.Helper()
	var ref string
	err := p.QueryRow(ctx, `SELECT coalesce(reference_transaction_id::text,'') FROM wager_transactions WHERE id=$1`, got.TransactionID).Scan(&ref)
	if err != nil || got.Status != wager.StatusProcessed || ref != want {
		t.Fatalf("result=%+v reference=%s want=%s err=%v", got, ref, want, err)
	}
}

func TestImplicitWINSelectsOldestUnsettledBET(t *testing.T) {
	p, ctx := accountingDB(t)
	w := accountingWallet(t, p, ctx, 10000)
	base := settlementTestClock{}.Now()
	round := uuid.NewString()
	// A lexicographically larger ID must still precede a newer operation.
	olderID := "f" + uuid.NewString()[1:]
	newerID := "0" + uuid.NewString()[1:]
	a := accountingInput(w, "BET", "20.00", "", round)
	b := accountingInput(w, "BET", "10.00", "", round)
	fifoSubmit(t, p, ctx, a, base.Add(time.Second), olderID)
	fifoSubmit(t, p, ctx, b, base.Add(2*time.Second), newerID)
	win := accountingInput(w, "WIN", "5.00", "", round)
	first := fifoSubmit(t, p, ctx, win, base.Add(5*time.Minute+3*time.Second), "")
	fifoReference(t, p, ctx, first, olderID)
	// Partial consumption does not make the next BET older or settle this one.
	second := fifoSubmit(t, p, ctx, accountingInput(w, "WIN", "15.00", "", round), base.Add(5*time.Minute+4*time.Second), "")
	fifoReference(t, p, ctx, second, olderID)
	third := fifoSubmit(t, p, ctx, accountingInput(w, "WIN", "10.00", "", round), base.Add(5*time.Minute+5*time.Second), "")
	fifoReference(t, p, ctx, third, newerID)
	replay := fifoSubmit(t, p, ctx, win, base.Add(5*time.Minute+6*time.Second), "")
	if !replay.IdempotentReplay || replay.TransactionID != first.TransactionID || replay.Balance.Minor() != 7500 {
		t.Fatalf("replay selected a new BET or balance: %+v", replay)
	}
	missing := fifoSubmit(t, p, ctx, accountingInput(w, "WIN", "1.00", "", round), base.Add(5*time.Minute+7*time.Second), "")
	if missing.Status != wager.StatusRejected || missing.FailureCode != wager.FailReferenceNotFound {
		t.Fatal("exhausted BET reused", missing)
	}
	accountingBalances(t, p, ctx, w.ID, 10000, 0)
	rec, err := usecase.NewReconcileWallet(NewUnitOfWork(p), metrics.New()).Execute(ctx, w.ID)
	if err != nil || !rec.Consistent || rec.CheckedEntries != 6 {
		t.Fatalf("ledger: %+v %v", rec, err)
	}
}

func TestImplicitWINSkipsClosedBetsAndOtherGames(t *testing.T) {
	p, ctx := accountingDB(t)
	w := accountingWallet(t, p, ctx, 10000)
	base := settlementTestClock{}.Now()
	round := uuid.NewString()
	closed := accountingInput(w, "BET", "20.00", "", round)
	c := fifoSubmit(t, p, ctx, closed, base.Add(time.Second), "")
	var betID string
	if err := p.QueryRow(ctx, `SELECT bet_id::text FROM wager_transactions WHERE id=$1`, c.TransactionID).Scan(&betID); err != nil {
		t.Fatal(err)
	}
	_, err := usecase.NewSettlements(NewUnitOfWork(p), accountingTestClock{base.Add(5*time.Minute + time.Second)}, settlementTestIDs{}).Confirm(ctx, betID, settlement.Distribution{ResultID: uuid.NewString(), Returns: []settlement.Return{{ExternalID: closed.ExternalTransactionID, Money: repoMoney(t, 2000)}}})
	if err != nil {
		t.Fatal(err)
	}
	foreign := accountingInput(w, "BET", "10.00", "", round)
	foreign.GameID = "other-game"
	fifoSubmit(t, p, ctx, foreign, base.Add(3*time.Second), "")
	eligible := fifoSubmit(t, p, ctx, accountingInput(w, "BET", "10.00", "", round), base.Add(4*time.Second), "")
	result := fifoSubmit(t, p, ctx, accountingInput(w, "WIN", "10.00", "", round), base.Add(5*time.Minute+5*time.Second), "")
	fifoReference(t, p, ctx, result, eligible.TransactionID)
	accountingBalances(t, p, ctx, w.ID, 7000, 3000)
}

func TestImplicitWINTimestampTieAndExplicitReference(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "timestamp-tie", true: "explicit-wins"}[explicit], func(t *testing.T) {
			p, ctx := accountingDB(t)
			w := accountingWallet(t, p, ctx, 10000)
			base := settlementTestClock{}.Now()
			round := uuid.NewString()
			low, high := "0"+uuid.NewString()[1:], "f"+uuid.NewString()[1:]
			highInput := accountingInput(w, "BET", "20.00", "", round)
			fifoSubmit(t, p, ctx, highInput, base.Add(time.Second), high)
			fifoSubmit(t, p, ctx, accountingInput(w, "BET", "20.00", "", round), base.Add(time.Second), low)
			ref, want := "", low
			if explicit {
				ref, want = highInput.ExternalTransactionID, high
			}
			got := fifoSubmit(t, p, ctx, accountingInput(w, "WIN", "20.00", ref, round), base.Add(5*time.Minute+2*time.Second), "")
			fifoReference(t, p, ctx, got, want)
		})
	}
}

func TestConcurrentImplicitWINsWaitForOldestBET(t *testing.T) {
	p, ctx := accountingDB(t)
	w := accountingWallet(t, p, ctx, 10000)
	base := settlementTestClock{}.Now()
	round := uuid.NewString()
	a := fifoSubmit(t, p, ctx, accountingInput(w, "BET", "20.00", "", round), base.Add(time.Second), "")
	b := fifoSubmit(t, p, ctx, accountingInput(w, "BET", "20.00", "", round), base.Add(2*time.Second), "")
	// Hold the oldest bet so both submissions reach the same locked candidate.
	lock, err := p.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Rollback(ctx) }()
	if _, err = lock.Exec(ctx, `SELECT b.id FROM bets b JOIN wager_transactions t ON t.bet_id=b.id WHERE t.id=$1 FOR UPDATE OF b`, a.TransactionID); err != nil {
		t.Fatal(err)
	}
	type outcome struct {
		result usecase.SubmitResult
		err    error
	}
	done := make(chan outcome, 2)
	for i := 0; i < 2; i++ {
		in := accountingInput(w, "WIN", "20.00", "", round)
		go func() {
			r, e := usecase.NewSubmitTransaction(NewUnitOfWork(p), accountingTestClock{base.Add(5*time.Minute + 3*time.Second)}, settlementTestIDs{}, metrics.New()).Execute(ctx, in)
			done <- outcome{r, e}
		}()
	}
	// Observe both server sessions blocked on the candidate query, not a sleep.
	for {
		var waiting int
		err = p.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%WITH candidate AS (%' AND query LIKE '%' || $1 || '%'`, "wt.kind='BET'").Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting == 2 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	if err = lock.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	refs := map[string]bool{}
	for i := 0; i < 2; i++ {
		out := <-done
		if out.err != nil || out.result.Status != wager.StatusProcessed {
			t.Fatalf("concurrent WIN: %+v %v", out.result, out.err)
		}
		var ref string
		if err = p.QueryRow(ctx, `SELECT reference_transaction_id::text FROM wager_transactions WHERE id=$1`, out.result.TransactionID).Scan(&ref); err != nil {
			t.Fatal(err)
		}
		refs[ref] = true
	}
	if len(refs) != 2 || !refs[a.TransactionID] || !refs[b.TransactionID] {
		t.Fatalf("same BET consumed twice: %v", refs)
	}
	accountingBalances(t, p, ctx, w.ID, 10000, 0)
}
