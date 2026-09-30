BEGIN;
DROP TRIGGER win_after_loss ON wager_transactions;
DROP FUNCTION accounting_win_after_loss_check();
DROP INDEX wager_processed_loss_context;
COMMIT;
