BEGIN READ ONLY;
EXPLAIN SELECT id, player_id, currency, balance_minor,
       balance_minor::numeric / 100 AS balance, version
FROM wallet_balances WHERE id = '00000000-0000-0000-0000-000000000000';
EXPLAIN SELECT id, origin, external_transaction_id, idempotency_key,
       kind, status, amount_minor, currency, failure_code,
       reference_external_id, reference_transaction_id
FROM wager_transactions
WHERE wallet_id = '00000000-0000-0000-0000-000000000000'
ORDER BY created_at, id;
EXPLAIN SELECT seq, transaction_id, direction, amount_minor,
       balance_before_minor, balance_after_minor,
       balance_after_minor = balance_before_minor +
         CASE direction WHEN 'CREDIT' THEN amount_minor ELSE -amount_minor END AS equation_ok
FROM wallet_ledger_entries
WHERE wallet_id = '00000000-0000-0000-0000-000000000000'
ORDER BY seq;
EXPLAIN SELECT w.id, w.balance_minor,
       COALESCE(SUM(CASE l.direction WHEN 'CREDIT' THEN l.amount_minor::numeric
                     ELSE -l.amount_minor::numeric END), 0) AS ledger_balance_minor,
       w.balance_minor - COALESCE(SUM(CASE l.direction WHEN 'CREDIT' THEN l.amount_minor::numeric
                     ELSE -l.amount_minor::numeric END), 0) AS difference_minor,
       COUNT(l.id) AS entries
FROM wallet_balances w LEFT JOIN wallet_ledger_entries l ON l.wallet_id = w.id
WHERE w.id = '00000000-0000-0000-0000-000000000000'
GROUP BY w.id, w.balance_minor;
EXPLAIN SELECT event_id, event_type, attempts, published_at, jsonb_pretty(payload)
FROM outbox_events WHERE aggregate_id = '00000000-0000-0000-0000-000000000000'
ORDER BY occurred_at, event_id;
EXPLAIN SELECT consumer_name, message_id, received_at, completed_at
FROM inbox_messages ORDER BY received_at DESC LIMIT 20;
ROLLBACK;
