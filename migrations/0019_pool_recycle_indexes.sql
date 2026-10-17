-- Bounded pool recycling and tenant-scoped audit queries. No business row changes.
CREATE INDEX IF NOT EXISTS idx_customer_recycle_cursor ON crm_customer (tenant_id, id) WHERE deleted_at IS NULL AND pool_status = 0 AND owner_user_id IS NOT NULL AND is_locked = false AND reassign_status = 0;
CREATE INDEX IF NOT EXISTS idx_opportunity_customer_active ON crm_opportunity (tenant_id, customer_id) WHERE deleted_at IS NULL AND status NOT IN ('won', 'lost');
CREATE INDEX IF NOT EXISTS idx_pool_log_tenant_created ON crm_pool_log (tenant_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_pool_log_customer_created ON crm_pool_log (tenant_id, customer_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;
