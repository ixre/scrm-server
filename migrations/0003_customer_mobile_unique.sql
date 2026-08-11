-- 客户手机号部分唯一索引（去重支撑）。
-- 只约束非空 mobile 且未软删的记录，避免存量重复数据阻塞迁移。
-- 同租户内同手机号视为重复（跨租户允许相同手机号）。
CREATE UNIQUE INDEX IF NOT EXISTS idx_customer_tenant_mobile
    ON crm_customer (tenant_id, mobile)
    WHERE mobile <> '' AND deleted_at IS NULL;
