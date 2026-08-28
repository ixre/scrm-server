-- Historical lead appointments remain unknown. Do not infer them from old notes.
ALTER TABLE crm_lead ADD COLUMN IF NOT EXISTS next_followup_at timestamptz;
ALTER TABLE crm_lead ADD COLUMN IF NOT EXISTS next_action varchar(512);
-- Lead followup indexes.
CREATE INDEX IF NOT EXISTS idx_lead_tenant_due_scan ON crm_lead (tenant_id, next_followup_at, id) WHERE deleted_at IS NULL AND pool_status = 0 AND convert_customer_id IS NULL AND (status IN ('new','contacted','') OR status IS NULL) AND next_followup_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_followup_tenant_lead_cursor ON crm_followup (tenant_id, lead_id, id DESC) WHERE deleted_at IS NULL;
