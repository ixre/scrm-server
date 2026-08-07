-- 手工迁移示例。
-- 背景：GORM AutoMigrate 只"新增"字段/表，改已有列定义（类型、长度、加/删索引等）做不到，
-- 这类改动在此按文件名升序写 SQL。每个文件在事务内执行，并以文件名作为版本号记录进
-- schema_migrations 表；已记录的文件重复运行不会再次执行（幂等）。
--
-- 示例（按需取消注释）：
--   -- 改列类型
--   ALTER TABLE staff MODIFY COLUMN name VARCHAR(255) NOT NULL COMMENT '员工姓名';
--   -- 新增列
--   ALTER TABLE customer ADD COLUMN remark VARCHAR(255) NULL COMMENT '备注' AFTER name;
--   -- 加索引
--   CREATE INDEX idx_customer_ext_corp_id ON customer (ext_corp_id);

-- 演示用无害语句，确认运行器可用后即可替换为真实 DDL。
SELECT 1;
