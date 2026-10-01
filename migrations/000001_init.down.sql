DROP TABLE IF EXISTS outbox_events;
DROP TABLE IF EXISTS inbox_messages;
DROP TRIGGER IF EXISTS trg_ledger_no_truncate ON wallet_ledger_entries;
DROP TRIGGER IF EXISTS trg_ledger_immutable ON wallet_ledger_entries;
DROP TABLE IF EXISTS wallet_ledger_entries;
DROP FUNCTION IF EXISTS forbid_ledger_mutation();
DROP TABLE IF EXISTS wager_transactions;
DROP TABLE IF EXISTS wallets;
