-- Bounded customer and contract pickers. Index-only, no historical data changes.
CREATE INDEX IF NOT EXISTS idx_customer_option_cursor ON crm_customer (tenant_id, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_customer_owner_option_cursor ON crm_customer (tenant_id, owner_user_id, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_customer_name_prefix ON crm_customer (tenant_id, lower(name) text_pattern_ops) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_customer_company_prefix ON crm_customer (tenant_id, lower(company) text_pattern_ops) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_customer_mobile_prefix ON crm_customer (tenant_id, lower(mobile) text_pattern_ops) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_order_option_cursor ON crm_order (tenant_id, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_order_user_option_cursor ON crm_order (tenant_id, user_id, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_order_title_prefix ON crm_order (tenant_id, lower(title) text_pattern_ops) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_order_number_prefix ON crm_order (tenant_id, lower(order_no) text_pattern_ops) WHERE deleted_at IS NULL;
