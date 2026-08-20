-- 老 openscrm 实体表回填 tenant_id（多租户隔离第二阶段 / 阶段 2B）。
--
-- 前置：本迁移前必须先跑过 cmd/migrate 里的 BackfillCRM，让 channel_link 表里有
-- (channel='wecom', entity_type='tenant', ext_id=corp_id) → entity_id=tenant_id 的映射。
-- BackfillCRM 以 staff 表的 distinct ext_corp_id 为种子建 tenant + 写 channel_link，
-- 故只要任意 corp 下有过 staff 即能反查到。
--
-- 策略：UPDATE FROM channel_link，仅回填 tenant_id IS NULL 的行。
-- 未匹配上的（孤儿）：ext_corp_id 在 channel_link 里查不到（极少见：只有 customer 而无 staff，
-- 或人工数据），保留 NULL 并记入 backfill_orphan 临时表供运维排查。
--
-- 顺序：customer 先（最关键），staff 次之（staff 也有 ext_corp_id，可独立回填）；
-- 其余表依赖 customer/staff 提供的 corp 映射，但 SQL 层面不依赖——7 个 UPDATE 互相独立。
-- ���表 UPDATE 不够原子安全：用 LIMIT 批处理避免长时间锁表。

-- 1) 回填 customer.tenant_id
UPDATE customer c
SET tenant_id = cl.entity_id
FROM channel_link cl
WHERE cl.channel = 'wecom'
  AND cl.entity_type = 'tenant'
  AND cl.ext_id = c.ext_corp_id
  AND c.tenant_id IS NULL;

-- 2) 回填 staff.tenant_id
UPDATE staff s
SET tenant_id = cl.entity_id
FROM channel_link cl
WHERE cl.channel = 'wecom'
  AND cl.entity_type = 'tenant'
  AND cl.ext_id = s.ext_corp_id
  AND s.tenant_id IS NULL;

-- 3) 回填 mass_msg.tenant_id
UPDATE mass_msg m
SET tenant_id = cl.entity_id
FROM channel_link cl
WHERE cl.channel = 'wecom'
  AND cl.entity_type = 'tenant'
  AND cl.ext_id = m.ext_corp_id
  AND m.tenant_id IS NULL;

-- 4) 回填 material.tenant_id
UPDATE material m
SET tenant_id = cl.entity_id
FROM channel_link cl
WHERE cl.channel = 'wecom'
  AND cl.entity_type = 'tenant'
  AND cl.ext_id = m.ext_corp_id
  AND m.tenant_id IS NULL;

-- 5) 回填 contact_way.tenant_id
UPDATE contact_way cw
SET tenant_id = cl.entity_id
FROM channel_link cl
WHERE cl.channel = 'wecom'
  AND cl.entity_type = 'tenant'
  AND cl.ext_id = cw.ext_corp_id
  AND cw.tenant_id IS NULL;

-- 6) 回填 group_chat.tenant_id
UPDATE group_chat gc
SET tenant_id = cl.entity_id
FROM channel_link cl
WHERE cl.channel = 'wecom'
  AND cl.entity_type = 'tenant'
  AND cl.ext_id = gc.ext_corp_id
  AND gc.tenant_id IS NULL;

-- 7) 回填 department.tenant_id
UPDATE department d
SET tenant_id = cl.entity_id
FROM channel_link cl
WHERE cl.channel = 'wecom'
  AND cl.entity_type = 'tenant'
  AND cl.ext_id = d.ext_corp_id
  AND d.tenant_id IS NULL;

-- 8) 孤儿审计：每张表 ext_corp_id 反查不到的行（含 tenant_id IS NULL + 非空 ext_corp_id）
-- 写入临时表，运维可拿这份名单手工建 corp 或修数据后再跑一次 0005（只补 NULL 的行）。
-- 注：临时表随连接释放消失，运维排查时建议复制成物理表：
--   CREATE TABLE backfill_orphan_audit AS SELECT * FROM pg_temp.backfill_orphan_audit;
DROP TABLE IF EXISTS backfill_orphan_audit;
CREATE TEMP TABLE backfill_orphan_audit AS
SELECT 'customer' AS tbl, id, ext_corp_id FROM customer WHERE ext_corp_id <> '' AND tenant_id IS NULL
UNION ALL SELECT 'staff', id, ext_corp_id FROM staff WHERE ext_corp_id <> '' AND tenant_id IS NULL
UNION ALL SELECT 'mass_msg', id, ext_corp_id FROM mass_msg WHERE ext_corp_id <> '' AND tenant_id IS NULL
UNION ALL SELECT 'material', id, ext_corp_id FROM material WHERE ext_corp_id <> '' AND tenant_id IS NULL
UNION ALL SELECT 'contact_way', id, ext_corp_id FROM contact_way WHERE ext_corp_id <> '' AND tenant_id IS NULL
UNION ALL SELECT 'group_chat', id, ext_corp_id FROM group_chat WHERE ext_corp_id <> '' AND tenant_id IS NULL
UNION ALL SELECT 'department', id, ext_corp_id FROM department WHERE ext_corp_id <> '' AND tenant_id IS NULL
