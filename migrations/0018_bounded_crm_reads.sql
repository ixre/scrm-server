-- Keyset pagination for customer histories and bounded employee pickers. No data changes or physical FKs.
CREATE INDEX IF NOT EXISTS idx_followup_customer_cursor ON crm_followup (tenant_id, customer_id, (COALESCE(created_at, 'epoch'::timestamptz)) DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_activity_customer_cursor ON crm_activity_record (tenant_id, customer_id, (COALESCE(happened_at, created_at, 'epoch'::timestamptz)) DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_interaction_customer_cursor ON crm_channel_interaction (tenant_id, customer_id, (COALESCE(happened_at, created_at, 'epoch'::timestamptz)) DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_transfer_customer_cursor ON crm_transfer_log (tenant_id, customer_id, (COALESCE(created_at, 'epoch'::timestamptz)) DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_user_tenant_status_cursor ON "user" (tenant_id, status, id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_user_tenant_cursor ON "user" (tenant_id, id) WHERE deleted_at IS NULL;
