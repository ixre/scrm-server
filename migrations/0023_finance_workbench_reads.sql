-- Tenant-scoped receipt prefix search and date scans independent of stored plan status.
CREATE INDEX IF NOT EXISTS idx_receipt_tenant_number_prefix ON crm_receivables (tenant_id, lower(receivables_no) text_pattern_ops) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_plan_tenant_calendar_scan ON crm_receivables_plan (tenant_id, plan_date, id DESC) WHERE deleted_at IS NULL AND plan_date IS NOT NULL;
