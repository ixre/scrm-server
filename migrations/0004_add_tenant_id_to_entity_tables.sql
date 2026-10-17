-- 老 openscrm 实体表加 tenant_id 列（多租户隔离第二阶段）。
-- 仅实体表加，关联表（customer_staff / mass_msg_staff / material_lib_tag 等）不加，
-- 关联表查询通过 JOIN 实体表带 tenant_id 过滤。
--
-- 列类型：bigint（与 crm_* 表、tenant.ID 保持一致；ID 在 GORM 层用 string 存储）。
-- 不加 NOT NULL：0005 仅为已有映射回填，旧渠道写入允许尚未映射到本地租户。
--
-- 索引：单列 B-tree 索引，支撑 WHERE tenant_id = ? 高频查询。
-- 0006/0014 保留与旧模型 ON CONFLICT 一致的业务唯一键，不将此可空映射作为业务键。

ALTER TABLE customer         ADD COLUMN IF NOT EXISTS tenant_id bigint;
ALTER TABLE staff            ADD COLUMN IF NOT EXISTS tenant_id bigint;
ALTER TABLE mass_msg         ADD COLUMN IF NOT EXISTS tenant_id bigint;
ALTER TABLE material         ADD COLUMN IF NOT EXISTS tenant_id bigint;
ALTER TABLE contact_way      ADD COLUMN IF NOT EXISTS tenant_id bigint;
ALTER TABLE group_chat       ADD COLUMN IF NOT EXISTS tenant_id bigint;
ALTER TABLE department       ADD COLUMN IF NOT EXISTS tenant_id bigint;

CREATE INDEX IF NOT EXISTS idx_customer_tenant_id    ON customer(tenant_id);
CREATE INDEX IF NOT EXISTS idx_staff_tenant_id       ON staff(tenant_id);
CREATE INDEX IF NOT EXISTS idx_mass_msg_tenant_id    ON mass_msg(tenant_id);
CREATE INDEX IF NOT EXISTS idx_material_tenant_id    ON material(tenant_id);
CREATE INDEX IF NOT EXISTS idx_contact_way_tenant_id ON contact_way(tenant_id);
CREATE INDEX IF NOT EXISTS idx_group_chat_tenant_id  ON group_chat(tenant_id);
CREATE INDEX IF NOT EXISTS idx_department_tenant_id  ON department(tenant_id);

COMMENT ON COLUMN customer.tenant_id    IS '租户ID（多租户隔离）';
COMMENT ON COLUMN staff.tenant_id       IS '租户ID（多租户隔离）';
COMMENT ON COLUMN mass_msg.tenant_id    IS '租户ID（多租户隔离）';
COMMENT ON COLUMN material.tenant_id    IS '租户ID（多租户隔离）';
COMMENT ON COLUMN contact_way.tenant_id IS '租户ID（多租户隔离）';
COMMENT ON COLUMN group_chat.tenant_id  IS '租户ID（多租户隔离）';
COMMENT ON COLUMN department.tenant_id  IS '租户ID（多租户隔离）';
