BEGIN;
DROP TRIGGER bet_window_check ON wager_transactions;
DROP FUNCTION accounting_bet_window_check();
ALTER TABLE bets DROP COLUMN betting_window_seconds;
COMMIT;
