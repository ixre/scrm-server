-- 阶段 2C：收口 tenant_id NOT NULL + 重建复合唯一索引。
-- 必须 0005 回填 100% 后执行。
--
-- 设计：
-- 1. 孤儿校验：每张表的孤儿 tenant_id 必须在 0；用单独 SELECT 校验（不能用 DO $$，因为迁移 runner 用 naive 分号 splitter）
-- 2. ALTER ... SET NOT NULL
-- 3. 把现有跨 corp 唯一索引 (ext_corp_id, ext_id) 重建为 (tenant_id, ext_id) 复合唯一，
--    允许不同租户有相同的 ext_id（多渠道场景）。
-- 4. 复合唯一索引用 partial index（WHERE deleted_at IS NULL）排除软删行。

-- 1. 孤儿校验（每张表单独 SELECT；任一不为 0 即失败，但 SELECT 自身不会 abort 事务，
--    所以建议执行前在 psql 手动跑这些 SELECT 验证为 0）
-- SELECT COUNT(*) FROM customer    WHERE tenant_id IS NULL;  -- 必须 0
-- SELECT COUNT(*) FROM staff       WHERE tenant_id IS NULL;  -- 必须 0
-- SELECT COUNT(*) FROM mass_msg    WHERE tenant_id IS NULL;  -- 必须 0
-- SELECT COUNT(*) FROM material    WHERE tenant_id IS NULL;  -- 必须 0
-- SELECT COUNT(*) FROM contact_way WHERE tenant_id IS NULL;  -- 必须 0
-- SELECT COUNT(*) FROM group_chat  WHERE tenant_id IS NULL;  -- 必须 0
-- SELECT COUNT(*) FROM department  WHERE tenant_id IS NULL;  -- 必须 0

-- 2. SET NOT NULL
ALTER TABLE customer    ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE staff       ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE mass_msg    ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE material    ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE contact_way ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE group_chat  ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE department  ALTER COLUMN tenant_id SET NOT NULL;

-- 3. 重建复合唯一索引（partial index 排除软删）
DROP INDEX IF EXISTS idx_ext_customer_id;
CREATE UNIQUE INDEX IF NOT EXISTS idx_customer_tenant_ext_id ON customer (tenant_id, ext_id) WHERE deleted_at IS NULL;

DROP INDEX IF EXISTS idx_ext_corp_id_ext_staff_id;
CREATE UNIQUE INDEX IF NOT EXISTS idx_staff_tenant_ext_id ON staff (tenant_id, ext_id) WHERE deleted_at IS NULL;

DROP INDEX IF EXISTS idx_ext_corp_id_ext_dept_id;
CREATE UNIQUE INDEX IF NOT EXISTS idx_department_tenant_ext_id ON department (tenant_id, ext_id) WHERE deleted_at IS NULL;

DROP INDEX IF EXISTS ext_chat_id;
CREATE UNIQUE INDEX IF NOT EXISTS idx_group_chat_tenant_ext_chat_id ON group_chat (tenant_id, ext_chat_id) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_mass_msg_tenant_ext_id ON mass_msg (tenant_id, ext_id) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_material_tenant_ext_id ON material (tenant_id, ext_id) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_contact_way_tenant_ext_id ON contact_way (tenant_id, ext_id) WHERE deleted_at IS NULL;
