-- Reminder occurrences are logical references, without physical foreign keys.
ALTER TABLE crm_todo ADD COLUMN IF NOT EXISTS source_id bigint;
ALTER TABLE crm_todo ADD COLUMN IF NOT EXISTS event_key varchar(160) NOT NULL DEFAULT '';
ALTER TABLE crm_todo ADD COLUMN IF NOT EXISTS handled_at timestamptz;
-- Legacy reminders cannot distinguish installments or handled occurrences.
-- Retain history, retire pending legacy reminders, regenerate from current sources.
UPDATE crm_todo SET status = 'cancelled', handled_at = CURRENT_TIMESTAMP
WHERE deleted_at IS NULL AND status = 'pending' AND event_key = ''
  AND type IN ('followup', 'receivables_expire');
-- Reminder indexes.
CREATE UNIQUE INDEX IF NOT EXISTS idx_todo_tenant_event ON crm_todo (tenant_id, event_key) WHERE event_key <> '';
CREATE INDEX IF NOT EXISTS idx_todo_tenant_user_status_due ON crm_todo (tenant_id, user_id, status, due_at, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_todo_tenant_pending_source ON crm_todo (tenant_id, type, source_id) WHERE deleted_at IS NULL AND status = 'pending';
CREATE INDEX IF NOT EXISTS idx_customer_tenant_owner_due ON crm_customer (tenant_id, owner_user_id, next_followup_at, id DESC) WHERE deleted_at IS NULL AND pool_status = 0 AND is_blacklist = false AND reassign_status = 0;
CREATE INDEX IF NOT EXISTS idx_customer_tenant_due_scan ON crm_customer (tenant_id, next_followup_at, id) WHERE deleted_at IS NULL AND pool_status = 0 AND is_blacklist = false AND reassign_status = 0 AND next_followup_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_plan_tenant_due_scan ON crm_receivables_plan (tenant_id, plan_date, id) WHERE deleted_at IS NULL AND status IN ('pending', 'overdue') AND plan_date IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_followup_tenant_user_created ON crm_followup (tenant_id, user_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;
