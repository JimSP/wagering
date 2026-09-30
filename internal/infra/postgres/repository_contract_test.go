package postgres

import (
	"context"
	"encoding/base64"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/domain/wallet"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	repoTime       = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	errRepoFailure = errors.New("database unavailable")
)

type testRow struct {
	values []any
	err    error
}

func (r testRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		panic("incorrect column count")
	}
	for i, v := range r.values {
		d := reflect.ValueOf(dest[i]).Elem()
		if v == nil {
			d.SetZero()
		} else {
			d.Set(reflect.ValueOf(v))
		}
	}
	return nil
}

type testRows struct {
	closeErr error
	pgx.Rows
	records []testRow
	index   int
	closed  bool
	err     error
}

func (r *testRows) Next() bool {
	if r.index >= len(r.records) {
		return false
	}
	r.index++
	return true
}
func (r *testRows) Scan(d ...any) error { return r.records[r.index-1].Scan(d...) }
func (r *testRows) Close() {
	if !r.closed {
		r.closed = true
		if r.err == nil {
			r.err = r.closeErr
		}
	}
}
func (r *testRows) Err() error { return r.err }

type testDB struct {
	tag   string
	err   error
	row   testRow
	rows  *testRows
	sql   string
	args  []any
	calls int
}

func (d *testDB) capture(sql string, a []any) { d.sql = sql; d.args = a; d.calls++ }
func (d *testDB) Exec(_ context.Context, s string, a ...any) (pgconn.CommandTag, error) {
	d.capture(s, a)
	return pgconn.NewCommandTag(d.tag), d.err
}

func (d *testDB) QueryRow(_ context.Context, s string, a ...any) pgx.Row {
	d.capture(s, a)
	return d.row
}

func (d *testDB) Query(_ context.Context, s string, a ...any) (pgx.Rows, error) {
	d.capture(s, a)
	return d.rows, d.err
}

func repoMoney(t *testing.T, n int64) money.Money {
	t.Helper()
	m, e := money.FromMinor(n, "BRL")
	if e != nil {
		t.Fatal(e)
	}
	return m
}

func requireError(t *testing.T, got, want error) {
	t.Helper()
	target := want
	if errors.Unwrap(want) != nil {
		target = errors.Unwrap(want)
	}
	if !errors.Is(got, target) {
		t.Fatalf("error=%v want=%v", got, want)
	}
	if want != nil && got.Error() != want.Error() {
		t.Fatalf("text=%q want=%q", got.Error(), want.Error())
	}
}

func requireArgs(t *testing.T, d *testDB, want ...any) {
	t.Helper()
	if !reflect.DeepEqual(d.args, want) {
		t.Fatalf("args=%#v want=%#v", d.args, want)
	}
}

func TestWalletPersistenceContract(t *testing.T) {
	ctx := context.Background()
	w, e := wallet.New("wallet", "player", repoMoney(t, 100), repoTime)
	if e != nil {
		t.Fatal(e)
	}
	d := &testDB{}
	r := walletRepo{d}
	requireError(t, r.Create(ctx, w), nil)
	s := w.Snapshot()
	requireArgs(t, d, s.ID, s.PlayerID, "BRL", int64(100), s.Version, repoTime, repoTime)
	for _, c := range []struct{ err, want error }{{errRepoFailure, errRepoFailure}, {&pgconn.PgError{Code: "23505"}, apperr.ErrWalletExists}, {&pgconn.PgError{Code: "23503"}, &pgconn.PgError{Code: "23503"}}} {
		d.err = c.err
		got := r.Create(ctx, w)
		if errors.Is(c.err, c.want) || errors.Is(c.want, apperr.ErrWalletExists) {
			requireError(t, got, c.want)
		} else if !errors.Is(got, c.err) {
			t.Fatal(got)
		}
	}
	d.err = nil
	for _, c := range []struct {
		tag       string
		err, want error
	}{{"UPDATE 1", nil, nil}, {"UPDATE 0", nil, apperr.ErrConcurrentModification}, {"UPDATE 2", nil, apperr.ErrConcurrentModification}, {"", errRepoFailure, errRepoFailure}} {
		d.tag = c.tag
		d.err = c.err
		requireError(t, r.Save(ctx, w, 7), c.want)
		requireArgs(t, d, "wallet", int64(100), s.Version, repoTime, int64(7))
	}
	d.err = nil
	d.row = testRow{values: []any{"wallet", "player", "BRL", int64(100), s.Version, repoTime, repoTime}}
	for _, lock := range []bool{false, true} {
		var got *wallet.Wallet
		var err error
		if lock {
			got, err = r.GetForUpdate(ctx, "wallet")
		} else {
			got, err = r.Get(ctx, "wallet")
		}
		requireError(t, err, nil)
		if got == nil || !reflect.DeepEqual(got.Snapshot(), s) {
			t.Fatal(got.Snapshot())
		}
		requireArgs(t, d, "wallet")
		if strings.Contains(d.sql, "FOR NO KEY UPDATE") != lock {
			t.Fatal(d.sql)
		}
	}
	for _, cause := range []error{pgx.ErrNoRows, errRepoFailure} {
		d.row = testRow{err: cause}
		got, err := r.Get(ctx, "wallet")
		want := cause
		if errors.Is(cause, pgx.ErrNoRows) {
			want = apperr.ErrWalletNotFound
		}
		requireError(t, err, want)
		if got != nil {
			t.Fatal(got)
		}
	}
	d.row = testRow{values: []any{"wallet", "player", "XXX", int64(100), s.Version, repoTime, repoTime}}
	got, err := r.Get(ctx, "wallet")
	requireError(t, err, money.ErrInvalidCurrency)
	if got != nil {
		t.Fatal(got)
	}
}

func opening(t *testing.T) *wager.Transaction {
	t.Helper()
	v, e := wager.NewOpening("tx", "wallet", "player", repoMoney(t, 100), repoTime)
	if e != nil {
		t.Fatal(e)
	}
	return v
}

func transactionRow(s wager.Snapshot) testRow {
	var b *int64
	if s.BalanceAfter != nil {
		v := s.BalanceAfter.Minor()
		b = &v
	}
	return testRow{values: []any{s.ID, s.Origin, s.ProviderID, s.ExternalID, s.IdempotencyKey, s.PayloadHash, s.WalletID, s.PlayerID, s.RoundID, s.GameID, s.Kind, s.Amount.Minor(), s.Amount.Currency(), s.ReferenceExternalID, s.ReferenceID, s.Status, s.FailureCode, b, s.Attempts, s.NextAttemptAt, s.ExpiresAt, s.CreatedAt, s.UpdatedAt, s.CorrelationID, s.CausationID}}
}

func TestTransactionPersistenceContract(t *testing.T) {
	ctx := context.Background()
	tx := opening(t)
	s := tx.Snapshot()
	d := &testDB{tag: "INSERT 0 1", row: transactionRow(s)}
	r := transactionRepo{d}
	requireError(t, r.Insert(ctx, tx), nil)
	if strings.Contains(d.sql, "ON CONFLICT") || len(d.args) != 25 || !reflect.DeepEqual(d.args, transactionRow(s).values) {
		t.Fatalf("insert args=%#v", d.args)
	}
	ok, err := r.InsertPending(ctx, tx)
	if !ok || err != nil || (!strings.HasPrefix(d.sql, "INSERT INTO wager_transactions(") || !strings.HasSuffix(d.sql, " ON CONFLICT DO NOTHING")) {
		t.Fatal(ok, err, d.sql)
	}
	d.tag = "INSERT 0 0"
	ok, err = r.InsertPending(ctx, tx)
	if ok || err != nil {
		t.Fatal(ok, err)
	}
	d.err = errRepoFailure
	requireError(t, r.Insert(ctx, tx), errRepoFailure)
	d.err = nil
	for _, get := range []func() (*wager.Transaction, error){func() (*wager.Transaction, error) { return r.FindByID(ctx, "tx") }, func() (*wager.Transaction, error) { return r.FindByIdempotencyKey(ctx, "provider", "key") }, func() (*wager.Transaction, error) { return r.FindByExternalID(ctx, "provider", "external") }} {
		got, e := get()
		requireError(t, e, nil)
		if got == nil || !reflect.DeepEqual(got.Snapshot(), s) {
			t.Fatal(got.Snapshot())
		}
	}
	for _, c := range []struct {
		tag       string
		err, want error
	}{{"UPDATE 1", nil, nil}, {"UPDATE 0", nil, apperr.ErrConcurrentModification}, {"UPDATE 2", nil, apperr.ErrConcurrentModification}, {"", errRepoFailure, errRepoFailure}} {
		d.tag = c.tag
		d.err = c.err
		requireError(t, r.Update(ctx, tx), c.want)
		requireArgs(t, d, s.ID, s.ReferenceID, s.Status, s.FailureCode, transactionRow(s).values[17], s.Attempts, s.NextAttemptAt, s.ExpiresAt, s.UpdatedAt)
	}
	d.err = nil
	d.rows = &testRows{records: []testRow{transactionRow(s)}}
	got, e := r.ClaimDue(ctx, repoTime, 20)
	requireError(t, e, nil)
	requireArgs(t, d, repoTime, 20)
	if len(got) != 1 || !reflect.DeepEqual(got[0].Snapshot(), s) || !d.rows.closed {
		t.Fatal(got)
	}
	for _, c := range []struct{ query, scan, iterate error }{{errRepoFailure, nil, nil}, {nil, errRepoFailure, nil}, {nil, nil, errRepoFailure}} {
		d.err = c.query
		d.rows = &testRows{err: c.iterate}
		if c.scan != nil {
			d.rows.records = []testRow{{err: c.scan}}
		}
		_, e = r.ClaimDue(ctx, repoTime, 20)
		requireError(t, e, errRepoFailure)
		if c.query == nil && !d.rows.closed {
			t.Fatal("rows leaked")
		}
	}
	d.err = nil
	for _, yes := range []bool{false, true} {
		d.row = testRow{values: []any{yes}}
		got, e := r.HasReversal(ctx, "ref")
		requireError(t, e, nil)
		if got != yes {
			t.Fatal(got)
		}
		requireArgs(t, d, "ref")
	}
	d.row = testRow{err: errRepoFailure}
	_, e = r.HasReversal(ctx, "ref")
	requireError(t, e, errRepoFailure)
	for _, cause := range []error{pgx.ErrNoRows, errRepoFailure} {
		v, e := scanTx(testRow{err: cause})
		want := cause
		if errors.Is(cause, pgx.ErrNoRows) {
			want = apperr.ErrNotFound
		}
		requireError(t, e, want)
		if v != nil {
			t.Fatal(v)
		}
	}
	bad := transactionRow(s)
	bad.values[12] = "XXX"
	v, e := scanTx(bad)
	requireError(t, e, money.ErrInvalidCurrency)
	if v != nil {
		t.Fatal(v)
	}
	// Pending transactions have no balance result; persistence must preserve nil.
	pending, e := wager.NewExternal(wager.ExternalParams{ID: "pending", ProviderID: "p", ExternalID: "e", IdempotencyKey: "k", PayloadHash: "h", WalletID: "w", PlayerID: "p", RoundID: "r", GameID: "g", Kind: wager.KindBet, Amount: repoMoney(t, 1)}, repoTime)
	requireError(t, e, nil)
	d.tag = "UPDATE 1"
	requireError(t, r.Update(ctx, pending), nil)
	if !reflect.ValueOf(d.args[4]).IsNil() {
		t.Fatal(d.args)
	}
	d.tag = "INSERT 0 1"
	requireError(t, r.Insert(ctx, pending), nil)
	if !reflect.ValueOf(d.args[17]).IsNil() {
		t.Fatal(d.args)
	}
	v, e = scanTx(transactionRow(pending.Snapshot()))
	requireError(t, e, nil)
	if v == nil || !reflect.DeepEqual(v.Snapshot(), pending.Snapshot()) {
		t.Fatal(v)
	}
}

func TestLedgerPaginationAndTotalsContract(t *testing.T) {
	ctx := context.Background()
	d := &testDB{}
	r := ledgerRepo{d}
	for _, limit := range []int{-1, 0, 201} {
		page, e := r.List(ctx, "w", "", limit)
		requireError(t, e, apperr.Invalid("limit out of range"))
		if page.Entries == nil || len(page.Entries) != 0 || page.NextCursor != "" {
			t.Fatal(page)
		}
	}
	for _, raw := range []string{"@", base64.RawURLEncoding.EncodeToString([]byte("w")), base64.RawURLEncoding.EncodeToString([]byte("other:1")), base64.RawURLEncoding.EncodeToString([]byte("w:x")), base64.RawURLEncoding.EncodeToString([]byte("w:-1"))} {
		_, e := r.List(ctx, "w", raw, 1)
		requireError(t, e, apperr.Invalid("invalid cursor"))
	}
	if d.calls != 0 {
		t.Fatal("invalid input reached DB")
	}
	record := func(id string, seq int64) testRow {
		return testRow{values: []any{id, "tx", wager.Credit, int64(100), "BRL", int64(0), int64(100), repoTime, seq}}
	}
	d.rows = &testRows{records: []testRow{record("first", 1)}}
	zeroPage, zeroErr := r.List(ctx, "w", base64.RawURLEncoding.EncodeToString([]byte("w:0")), 1)
	requireError(t, zeroErr, nil)
	requireArgs(t, d, "w", int64(0), 2)
	if len(zeroPage.Entries) != 1 || zeroPage.Entries[0].ID() != "first" || zeroPage.NextCursor != "" {
		t.Fatal(zeroPage)
	}
	d.rows = &testRows{records: []testRow{record("l1", 7), record("l2", 8)}}
	page, e := r.List(ctx, "w", "", 1)
	requireError(t, e, nil)
	if len(page.Entries) != 1 || page.Entries[0].ID() != "l1" || page.Entries[0].BalanceAfter().Minor() != 100 || page.NextCursor != base64.RawURLEncoding.EncodeToString([]byte("w:7")) || !d.rows.closed {
		t.Fatal(page)
	}
	requireArgs(t, d, "w", int64(0), 2)
	d.rows = &testRows{records: []testRow{record("l2", 8)}}
	next, e := r.List(ctx, "w", page.NextCursor, 200)
	requireError(t, e, nil)
	requireArgs(t, d, "w", int64(7), 201)
	if len(next.Entries) != 1 || next.NextCursor != "" {
		t.Fatal(next)
	}
	requireError(t, r.Append(ctx, page.Entries[0]), nil)
	requireArgs(t, d, "l1", "tx", repoTime)
	d.err = errRepoFailure
	requireError(t, r.Append(ctx, page.Entries[0]), errRepoFailure)
	for _, c := range []struct{ query, scan, iterate error }{{errRepoFailure, nil, nil}, {nil, errRepoFailure, nil}, {nil, nil, errRepoFailure}} {
		d.err = c.query
		d.rows = &testRows{err: c.iterate}
		if c.scan != nil {
			d.rows.records = []testRow{{err: c.scan}}
		}
		_, e = r.List(ctx, "w", "", 1)
		requireError(t, e, errRepoFailure)
		if c.query == nil && !d.rows.closed {
			t.Fatal("rows leaked")
		}
	}
	d.err = nil
	bad := record("bad", 1)
	bad.values[4] = "XXX"
	d.rows = &testRows{records: []testRow{bad}}
	_, e = r.List(ctx, "w", "", 1)
	requireError(t, e, money.ErrInvalidCurrency)
	bad = record("bad", 1)
	bad.values[6] = int64(101)
	d.rows = &testRows{records: []testRow{bad}}
	_, e = r.List(ctx, "w", "", 1)
	if e == nil || !strings.Contains(e.Error(), "ledger") {
		t.Fatal(e)
	}
	d.row = testRow{values: []any{"BRL", "100", int64(2)}}
	tot, e := r.Totals(ctx, "w")
	requireError(t, e, nil)
	if tot.Calculated != repoMoney(t, 100) || tot.Entries != 2 {
		t.Fatal(tot)
	}
	d.row = testRow{err: errRepoFailure}
	_, e = r.Totals(ctx, "w")
	requireError(t, e, errRepoFailure)
	d.row = testRow{values: []any{"BRL", "9223372036854775808", int64(2)}}
	_, e = r.Totals(ctx, "w")
	requireError(t, e, money.ErrOverflow)
	d.row = testRow{values: []any{"XXX", "1", int64(2)}}
	_, e = r.Totals(ctx, "w")
	requireError(t, e, money.ErrInvalidCurrency)
}

func TestLedgerReportsErrorDiscoveredWhileClosingLookahead(t *testing.T) {
	row := func(id string, seq int64) testRow {
		return testRow{values: []any{id, "tx", wager.Credit, int64(100), "BRL", int64(0), int64(100), repoTime, seq}}
	}
	d := &testDB{rows: &testRows{records: []testRow{row("one", 1), row("two", 2)}, closeErr: errRepoFailure}}
	_, err := (ledgerRepo{d}).List(context.Background(), "w", "", 1)
	requireError(t, err, errRepoFailure)
	if !d.rows.closed {
		t.Fatal("rows leaked")
	}
}
