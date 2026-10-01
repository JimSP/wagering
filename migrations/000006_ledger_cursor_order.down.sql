BEGIN;
CREATE OR REPLACE FUNCTION accounting_entry_guard() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog,public,pg_temp AS $$
DECLARE a ledger_accounts; j ledger_journals; p ledger_entries; t wager_transactions; expected BIGINT;
BEGIN
 SELECT * INTO STRICT j FROM ledger_journals WHERE id=NEW.journal_id;
 IF j.created_xid<>pg_current_xact_id() THEN RAISE EXCEPTION 'journal sealed'; END IF;
 SELECT * INTO STRICT a FROM ledger_accounts WHERE id=NEW.account_id FOR NO KEY UPDATE;
 SELECT * INTO STRICT t FROM wager_transactions WHERE id=j.transaction_id;
 SELECT * INTO p FROM ledger_entries WHERE account_id=a.id ORDER BY account_version DESC LIMIT 1;
 expected:=CASE WHEN t.kind='OPENING' THEN 1 ELSE coalesce(p.account_version,1)+1 END;
 IF NEW.balance_before_minor<>coalesce(p.balance_after_minor,0) OR NEW.account_version<>expected
 OR NEW.currency<>j.currency OR NEW.amount_minor<>j.amount_minor THEN RAISE EXCEPTION 'invalid posting chain or journal amount'; END IF;
 RETURN NEW;
END $$;
COMMIT;
