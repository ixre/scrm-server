-- 0007: 数据库模型设计规范化清理
--
-- 本迁移配合 crm.go 模型重构，处理以下问题：
-- 1. 删除 crm_workorder.created_at2 冗余字段（与 created_at 重复）
-- 2. 删除 crm_knowledge.category_id / crm_knowledge.public_type 重复字段
-- 3. 创建 crm_product_category 表（由 AutoMigrate 自动创建）
-- 4. 统一状态字段类型：crm_income_category.status smallint→boolean
-- 5. 统一状态字段类型：crm_workorder_type.status smallint→boolean
-- 6. 统一状态字段类型：crm_aftersales_type.status smallint→boolean
-- 7. 删除 crm_customer_setting.recycle_days 冗余字段（由 crm_pool_rule.recycle_days 统一管理）
--
-- 注意：
-- - AutoMigrate 会自动新增字段/表，但不会删除字段或修改列类型，
--   因此 DROP COLUMN / ALTER COLUMN TYPE 必须在本迁移中显式执行。
-- - prev_owner_user_id / operator_id / target_user_id 保持 varchar(64)，
--   因为现有代码用空字符串表示"无值"（与 owner_user_id 一致），改为 bigint 会破坏此约定。

-- 1. 删除 crm_workorder.created_at2
ALTER TABLE crm_workorder DROP COLUMN IF EXISTS created_at2;

-- 2. 删除 crm_knowledge 重复字段
ALTER TABLE crm_knowledge DROP COLUMN IF EXISTS category_id;
ALTER TABLE crm_knowledge DROP COLUMN IF EXISTS public_type;

-- 3. crm_product_category 表由 AutoMigrate 自动创建，此处无需手动建表

-- 4. crm_income_category.status smallint→boolean
ALTER TABLE crm_income_category ALTER COLUMN status DROP DEFAULT;
ALTER TABLE crm_income_category ALTER COLUMN status TYPE boolean USING status::text::boolean;
ALTER TABLE crm_income_category ALTER COLUMN status SET DEFAULT true;

-- 5. crm_workorder_type.status smallint→boolean
ALTER TABLE crm_workorder_type ALTER COLUMN status DROP DEFAULT;
ALTER TABLE crm_workorder_type ALTER COLUMN status TYPE boolean USING status::text::boolean;
ALTER TABLE crm_workorder_type ALTER COLUMN status SET DEFAULT true;

-- 6. crm_aftersales_type.status smallint→boolean
ALTER TABLE crm_aftersales_type ALTER COLUMN status DROP DEFAULT;
ALTER TABLE crm_aftersales_type ALTER COLUMN status TYPE boolean USING status::text::boolean;
ALTER TABLE crm_aftersales_type ALTER COLUMN status SET DEFAULT true;

-- 7. 删除 crm_customer_setting.recycle_days
ALTER TABLE crm_customer_setting DROP COLUMN IF EXISTS recycle_days;
