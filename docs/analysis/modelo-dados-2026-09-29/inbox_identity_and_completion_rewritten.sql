SET ROLE wagering_app;
BEGIN;
INSERT INTO inbox_messages(consumer_name,message_id,payload_hash,received_at,completed_at) VALUES('probe','d4eef46a-466e-4b56-9936-9dfb36c07f44','original','2026-09-29T12:00:00Z','2026-09-29T12:00:00Z'); UPDATE inbox_messages SET payload_hash='other',completed_at=NULL WHERE consumer_name='probe';
SET CONSTRAINTS ALL IMMEDIATE;
ROLLBACK;
