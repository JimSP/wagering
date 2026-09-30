package postgres

import (
	"context"
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/domain/money"
	"github.com/alexandre/wagering/internal/domain/wager"
)

type ledgerRepo struct{ q dbtx }

func (r ledgerRepo) Append(ctx context.Context, e wager.LedgerEntry) error {
	_, err := r.q.Exec(ctx, `SELECT accounting_opening_entry($1::uuid,$2::uuid,$3)`, e.ID(), e.TransactionID(), e.CreatedAt())
	return err
}

func (r ledgerRepo) List(ctx context.Context, w, cursor string, limit int) (port.LedgerPage, error) {
	var seq int64
	out := port.LedgerPage{Entries: []wager.LedgerEntry{}}
	if limit < 1 || limit > 200 {
		return out, apperr.Invalid("limit out of range")
	}
	if cursor != "" {
		b, e := base64.RawURLEncoding.DecodeString(cursor)
		if e != nil {
			return out, apperr.Invalid("invalid cursor")
		}
		parts := strings.Split(string(b), ":")
		if len(parts) != 2 || parts[0] != w {
			return out, apperr.Invalid("invalid cursor")
		}
		seq, e = strconv.ParseInt(parts[1], 10, 64)
		if e != nil || seq < 0 {
			return out, apperr.Invalid("invalid cursor")
		}
	}
	rows, err := r.q.Query(ctx, `SELECT id::text,transaction_id::text,direction,amount_minor,currency,balance_before_minor,balance_after_minor,created_at,seq FROM wallet_ledger_entries WHERE wallet_id=$1 AND seq>$2 ORDER BY seq LIMIT $3`, w, seq, limit+1)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	var last int64
	for rows.Next() {
		var id, tx, cur string
		var dir wager.Direction
		var amount, before, after, s int64
		var at time.Time
		if err = rows.Scan(&id, &tx, &dir, &amount, &cur, &before, &after, &at, &s); err != nil {
			return out, err
		}
		if len(out.Entries) == limit {
			out.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(w + ":" + strconv.FormatInt(last, 10)))
			// Closing can discover a server error after the lookahead row.
			rows.Close()
			return out, rows.Err()
		}
		a, e := money.FromMinor(amount, cur)
		if e != nil {
			return out, e
		}
		b, e := money.FromMinor(before, cur)
		if e != nil {
			return out, e
		}
		c, e := money.FromMinor(after, cur)
		if e != nil {
			return out, e
		}
		entry, e := wager.RehydrateLedgerEntry(id, w, tx, dir, a, b, c, at)
		if e != nil {
			return out, e
		}
		out.Entries = append(out.Entries, entry)
		last = s
	}
	return out, rows.Err()
}

func (r ledgerRepo) Totals(ctx context.Context, w string) (port.LedgerTotals, error) {
	var total string
	var n int64
	var cur string
	err := r.q.QueryRow(ctx, `SELECT w.currency,coalesce(sum(CASE l.direction WHEN 'CREDIT' THEN l.amount_minor::numeric ELSE -l.amount_minor::numeric END),0)::text,count(l.id) FROM wallets w LEFT JOIN wallet_ledger_entries l ON l.wallet_id=w.id WHERE w.id=$1 GROUP BY w.id`, w).Scan(&cur, &total, &n)
	if err != nil {
		return port.LedgerTotals{}, err
	}
	v, err := strconv.ParseInt(total, 10, 64)
	if err != nil {
		return port.LedgerTotals{}, money.ErrOverflow
	}
	m, err := money.FromMinor(v, cur)
	return port.LedgerTotals{Calculated: m, Entries: n}, err
}
