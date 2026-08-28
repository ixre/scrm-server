-- Opportunity selection by customer and bounded open pipeline cards/summaries.
CREATE INDEX IF NOT EXISTS idx_opportunity_customer_title_prefix ON crm_opportunity (tenant_id, customer_id, lower(title) text_pattern_ops) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_opportunity_open_stage_cursor ON crm_opportunity (tenant_id, stage_id, id DESC) INCLUDE (amount, probability, expected_close_date) WHERE deleted_at IS NULL AND status = 'open';
CREATE INDEX IF NOT EXISTS idx_opportunity_open_owner_stage_cursor ON crm_opportunity (tenant_id, owner_user_id, stage_id, id DESC) INCLUDE (amount, probability, expected_close_date) WHERE deleted_at IS NULL AND status = 'open';
CREATE INDEX IF NOT EXISTS idx_opportunity_open_calendar ON crm_opportunity (tenant_id, expected_close_date, id DESC) INCLUDE (stage_id, owner_user_id, amount, probability) WHERE deleted_at IS NULL AND status = 'open';
CREATE INDEX IF NOT EXISTS idx_opportunity_open_owner_calendar ON crm_opportunity (tenant_id, owner_user_id, expected_close_date, id DESC) INCLUDE (stage_id, amount, probability) WHERE deleted_at IS NULL AND status = 'open';
