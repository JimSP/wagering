BEGIN;
DROP TRIGGER early_win_check ON wager_transactions;
DROP FUNCTION accounting_early_win_check();
COMMIT;
