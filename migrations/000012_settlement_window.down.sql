BEGIN;
DROP TRIGGER settlement_window_check ON settlements;
DROP FUNCTION accounting_settlement_window_check();
COMMIT;
