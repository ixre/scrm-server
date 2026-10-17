-- Preserve existing tasks and history. No physical foreign keys.
ALTER TABLE crm_channel_task ADD COLUMN IF NOT EXISTS due_at timestamptz;
ALTER TABLE crm_channel_task ADD COLUMN IF NOT EXISTS handled_at timestamptz;
ALTER TABLE crm_channel_task ADD COLUMN IF NOT EXISTS handled_by bigint;
ALTER TABLE crm_channel_task ADD COLUMN IF NOT EXISTS resolution varchar(512);
ALTER TABLE crm_channel_task ADD COLUMN IF NOT EXISTS version bigint NOT NULL DEFAULT 1;
ALTER TABLE crm_channel_task ADD COLUMN IF NOT EXISTS dedup_key varchar(160) NOT NULL DEFAULT '';
UPDATE crm_channel_task SET due_at = created_at + INTERVAL '48 hours' WHERE due_at IS NULL AND created_at IS NOT NULL;
ALTER TABLE crm_channel_interaction ADD COLUMN IF NOT EXISTS event_key varchar(64) NOT NULL DEFAULT '';
CREATE TABLE IF NOT EXISTS crm_channel_task_log (
 id bigint PRIMARY KEY, tenant_id bigint NOT NULL, task_id bigint NOT NULL, actor_id bigint,
 action varchar(32) NOT NULL, from_status varchar(16), to_status varchar(16),
 from_user_id bigint, to_user_id bigint, note varchar(512),
 created_at timestamptz, updated_at timestamptz, deleted_at timestamptz
);
-- Channel task indexes.
CREATE UNIQUE INDEX IF NOT EXISTS idx_channel_task_event ON crm_channel_task (tenant_id, dedup_key) WHERE dedup_key <> '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_channel_interaction_event ON crm_channel_interaction (tenant_id, event_key) WHERE event_key <> '';
CREATE INDEX IF NOT EXISTS idx_channel_task_tenant_assignee_status ON crm_channel_task (tenant_id, assign_user_id, status, created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_channel_task_tenant_status ON crm_channel_task (tenant_id, status, created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_channel_task_tenant_customer_action ON crm_channel_task (tenant_id, customer_id, action, from_channel, to_channel, created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_channel_task_due ON crm_channel_task (due_at, id) WHERE deleted_at IS NULL AND status = 'pending';
CREATE INDEX IF NOT EXISTS idx_channel_task_log_timeline ON crm_channel_task_log (tenant_id, task_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_customer_channel_identity ON crm_customer (tenant_id, source, ext_corp_id, ext_customer_id) WHERE deleted_at IS NULL;
