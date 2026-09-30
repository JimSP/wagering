import { PGlite } from '../.local/sqlcheck/node_modules/@electric-sql/pglite/dist/index.js';
import fs from 'node:fs';
const db=new PGlite();
const root=new URL('../migrations/',import.meta.url).pathname;
for(const name of ['000001_init.up.sql','000002_integrity.up.sql','000002_integrity.down.sql','000001_init.down.sql','000001_init.up.sql','000002_integrity.up.sql']){await db.exec(fs.readFileSync(root+name,'utf8'));console.log('OK',name)}
const wallet='00000000-0000-4000-8000-000000000001', player='00000000-0000-4000-8000-000000000002', tx='00000000-0000-4000-8000-000000000003';
await db.exec(`BEGIN;
INSERT INTO wallets VALUES('${wallet}','${player}','BRL',10000,1,now(),now());
INSERT INTO wager_transactions(id,origin,wallet_id,player_id,kind,amount_minor,currency,status,balance_after_minor,created_at,updated_at) VALUES('${tx}','INTERNAL','${wallet}','${player}','OPENING',10000,'BRL','PROCESSED',10000,now(),now());
INSERT INTO wallet_ledger_entries(id,wallet_id,transaction_id,direction,amount_minor,currency,balance_before_minor,balance_after_minor,created_at) VALUES(gen_random_uuid(),'${wallet}','${tx}','CREDIT',10000,'BRL',0,10000,now());
INSERT INTO outbox_events(event_id,aggregate_id,event_type,payload,occurred_at,next_attempt_at) SELECT gen_random_uuid(),'${wallet}',e,jsonb_build_object('data',jsonb_build_object('transactionId','${tx}')),now(),now() FROM unnest(ARRAY['WagerTransactionProcessed','WalletBalanceChanged']) e;
COMMIT;`);console.log('OK atomic opening');
for (const sql of [`UPDATE wallets SET balance_minor=1,version=2 WHERE id='${wallet}'`,`DELETE FROM wallet_ledger_entries`,`UPDATE wallet_ledger_entries SET amount_minor=1`,`TRUNCATE wallet_ledger_entries`]){let rejected=false;try{await db.exec(sql)}catch(e){rejected=true;console.log('REJECTED',e.message)}if(!rejected)throw Error('mutation accepted '+sql)}
const bet='00000000-0000-4000-8000-000000000004';
await db.exec(`BEGIN;
INSERT INTO wager_transactions(id,origin,provider_id,external_transaction_id,idempotency_key,payload_hash,wallet_id,player_id,round_id,game_id,kind,amount_minor,currency,status,created_at,updated_at) VALUES('${bet}','EXTERNAL','p','bet','key','hash','${wallet}','${player}','r','g','BET',8000,'BRL','PENDING',now(),now());
SELECT * FROM wallets WHERE id='${wallet}' FOR NO KEY UPDATE;
INSERT INTO wallet_ledger_entries(id,wallet_id,transaction_id,direction,amount_minor,currency,balance_before_minor,balance_after_minor,created_at) VALUES(gen_random_uuid(),'${wallet}','${bet}','DEBIT',8000,'BRL',10000,2000,now());
UPDATE wallets SET balance_minor=2000,version=2 WHERE id='${wallet}';
UPDATE wager_transactions SET status='PROCESSED',balance_after_minor=2000 WHERE id='${bet}';
INSERT INTO outbox_events(event_id,aggregate_id,event_type,payload,occurred_at,next_attempt_at) SELECT gen_random_uuid(),'${wallet}',e,jsonb_build_object('data',jsonb_build_object('transactionId','${bet}')),now(),now() FROM unnest(ARRAY['WagerTransactionProcessed','WalletBalanceChanged']) e;
COMMIT;`);console.log('OK atomic bet; balance', (await db.query('SELECT balance_minor::text,version FROM wallets')).rows);
await db.close();
