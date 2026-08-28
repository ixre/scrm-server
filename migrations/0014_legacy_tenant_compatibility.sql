-- 修复已登记旧 0006 的库。与修正版 0006 使用相同兼容性 DDL，可重复执行。
-- 旧企微模型兼容迁移。保留 0006 文件名作为历史版本号。
-- tenant_id 是可空的 CRM 映射，旧渠道读写仍使用 ext_corp_id / 外部业务键。
-- 不新增虚构 ext_id，不把未映射数据归入默认租户，不改变已有 tenant_id 值。
-- 唯一索引必须与 Customer/Staff/Department/GroupChat 的 ON CONFLICT 一致。
-- 恢复全量唯一键若遇到历史冲突会失败，事务回滚，禁止自动合并或删除记录。

ALTER TABLE customer    ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE staff       ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE mass_msg    ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE material    ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE contact_way ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE group_chat  ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE department  ALTER COLUMN tenant_id DROP NOT NULL;

-- 先恢复运行中的写入依赖，再移除旧 0006 引入的不兼容约束。
CREATE UNIQUE INDEX IF NOT EXISTS idx_ext_customer_id ON customer (ext_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ext_corp_id_ext_staff_id ON staff (ext_corp_id, ext_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_ext_corp_id_ext_dept_id ON department (ext_corp_id, ext_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_group_chat_ext_chat_id ON group_chat (ext_chat_id);

DROP INDEX IF EXISTS idx_customer_tenant_ext_id;
DROP INDEX IF EXISTS idx_staff_tenant_ext_id;
DROP INDEX IF EXISTS idx_department_tenant_ext_id;
DROP INDEX IF EXISTS idx_group_chat_tenant_ext_chat_id;
DROP INDEX IF EXISTS idx_mass_msg_tenant_ext_id;
DROP INDEX IF EXISTS idx_material_tenant_ext_id;
DROP INDEX IF EXISTS idx_contact_way_tenant_ext_id;
