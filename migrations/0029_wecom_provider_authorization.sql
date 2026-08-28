-- migrate:statement
ALTER TABLE crm_channel_config ADD COLUMN IF NOT EXISTS auth_mode varchar(32) NOT NULL DEFAULT 'self_build';
-- migrate:statement
ALTER TABLE crm_channel_config ADD COLUMN IF NOT EXISTS suite_id varchar(64);
-- migrate:statement
ALTER TABLE crm_channel_config ADD COLUMN IF NOT EXISTS permanent_code text;
-- migrate:statement
ALTER TABLE crm_channel_config ADD COLUMN IF NOT EXISTS auth_status varchar(32);
-- migrate:statement
ALTER TABLE crm_channel_config ADD COLUMN IF NOT EXISTS authorized_at bigint NOT NULL DEFAULT 0;
-- migrate:statement
CREATE TABLE IF NOT EXISTS wecom_suite_ticket(suite_id varchar(64) PRIMARY KEY,ticket text NOT NULL,timestamp bigint NOT NULL);
-- migrate:statement
CREATE TABLE IF NOT EXISTS wecom_auth_state(state_hash varchar(64) PRIMARY KEY,tenant_id bigint NOT NULL,user_id bigint NOT NULL,suite_id varchar(64) NOT NULL,expires_at timestamp with time zone NOT NULL,auth_code text,corp_id varchar(64),completed_at timestamp with time zone,attempts integer NOT NULL DEFAULT 0,last_error text);
-- migrate:statement
CREATE INDEX IF NOT EXISTS idx_wecom_auth_pending ON wecom_auth_state(expires_at) WHERE completed_at IS NULL;
