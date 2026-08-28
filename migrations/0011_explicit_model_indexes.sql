-- Indexes match scoped list/detail queries. Business IDs remain logical references.
-- Keep existing indexes until production query statistics justify removing them.
-- btree_gin allows tenant equality and JSON membership to share one index.
CREATE EXTENSION IF NOT EXISTS btree_gin;
CREATE INDEX IF NOT EXISTS idx_customer_tenant_owner_score ON crm_customer (tenant_id, owner_user_id, score DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_customer_tenant_pool_score ON crm_customer (tenant_id, pool_status, score DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_lead_tenant_owner_status_id ON crm_lead (tenant_id, owner_user_id, status, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_opportunity_tenant_owner_id ON crm_opportunity (tenant_id, owner_user_id, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_followup_tenant_customer_timeline ON crm_followup (tenant_id, customer_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_order_tenant_customer_created ON crm_order (tenant_id, customer_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_receivables_tenant_order ON crm_receivables (tenant_id, order_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_quote_tenant_customer_created ON crm_quote (tenant_id, customer_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_contact_staff_corp_parent ON contact_way_staff (ext_corp_id, contact_way_id, id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_contact_backup_corp_parent ON contact_way_backup_staff (ext_corp_id, contact_way_id, id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_contact_schedule_corp_parent ON contact_way_schedule (ext_corp_id, contact_way_id, id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_schedule_staff_corp_parent ON contact_way_schedule_staff (ext_corp_id, contact_way_schedule_id, id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_quick_reply_corp_group ON quick_reply (ext_corp_id, group_id, id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_quick_detail_corp_parent ON quick_reply_detail (ext_corp_id, quick_reply_id, id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_welcome_corp_parent ON welcome_msg (ext_corp_id, main_welcome_msg_id, id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_mass_staff_corp_parent ON mass_msg_staff (ext_corp_id, mass_msg_id, id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_customer_tenant_score ON crm_customer (tenant_id, score DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_customer_tenant_shared_staffs_gin ON crm_customer USING gin (tenant_id, shared_staff_ids jsonb_path_ops) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_contact_staff_corp_employee ON contact_way_staff (ext_corp_id, ext_staff_id, contact_way_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_contact_backup_corp_employee ON contact_way_backup_staff (ext_corp_id, ext_staff_id, contact_way_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_schedule_staff_corp_employee ON contact_way_schedule_staff (ext_corp_id, ext_staff_id, contact_way_id) WHERE deleted_at IS NULL;
