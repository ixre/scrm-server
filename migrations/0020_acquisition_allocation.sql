-- Existing rules remain configuration-only until explicitly opted in.
ALTER TABLE crm_auto_allocation ADD COLUMN IF NOT EXISTS execution_enabled boolean NOT NULL DEFAULT false;
ALTER TABLE crm_auto_allocation ADD COLUMN IF NOT EXISTS sources jsonb;
ALTER TABLE crm_auto_allocation ADD COLUMN IF NOT EXISTS last_target_id varchar(64);
CREATE TABLE IF NOT EXISTS crm_allocation_log (
 id bigint PRIMARY KEY, created_at timestamptz, updated_at timestamptz, deleted_at timestamptz,
 tenant_id bigint, customer_id bigint, customer_name varchar(128), rule_id bigint,
 rule_name varchar(128), source varchar(32), target_user_id varchar(64), outcome varchar(32), reason varchar(512)
);
-- Allocation indexes.
CREATE INDEX IF NOT EXISTS idx_allocation_execution ON crm_auto_allocation (tenant_id, priority DESC, id DESC) WHERE deleted_at IS NULL AND entity_type = 'customer' AND is_enabled = true AND execution_enabled = true;
CREATE UNIQUE INDEX IF NOT EXISTS idx_allocation_customer_once ON crm_allocation_log (tenant_id, customer_id);
CREATE INDEX IF NOT EXISTS idx_allocation_log_tenant_time ON crm_allocation_log (tenant_id, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_allocation_log_rule_time ON crm_allocation_log (tenant_id, rule_id, id DESC) WHERE deleted_at IS NULL;
