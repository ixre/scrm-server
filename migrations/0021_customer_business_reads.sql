-- Tenant/customer-scoped business summaries and ID cursors. No data changes or physical FKs.
CREATE INDEX IF NOT EXISTS idx_order_customer_business_cursor ON crm_order (tenant_id, customer_id, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_order_customer_user_business_cursor ON crm_order (tenant_id, customer_id, user_id, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_quote_customer_business_cursor ON crm_quote (tenant_id, customer_id, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_quote_customer_owner_business_cursor ON crm_quote (tenant_id, customer_id, owner_user_id, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_opportunity_customer_business_cursor ON crm_opportunity (tenant_id, customer_id, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_opportunity_customer_owner_business_cursor ON crm_opportunity (tenant_id, customer_id, owner_user_id, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_plan_order_business_cursor ON crm_receivables_plan (tenant_id, order_id, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_receipt_order_business_cursor ON crm_receivables (tenant_id, order_id, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_receipt_plan_effective_sum ON crm_receivables (tenant_id, order_id, plan_id) INCLUDE (amount) WHERE deleted_at IS NULL AND status = 'confirmed' AND (approve_status IS NULL OR approve_status IN ('','none','approved'));
