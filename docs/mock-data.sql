-- =============================================================================
-- mock-data.sql — 演示租户 CRM 与企微员工后台测试数据
-- -----------------------------------------------------------------------------
-- 用途：为 dashboard（已从 antd 迁移到 Arco）提供可展示、可交互的测试数据。
-- 覆盖两套数据：
--   1) CRM 模块（demo 租户 tenant.id = 2087127714048774144，登录 admin/123456）
--   2) 企微员工后台（wework 强制登录为 WangQiang，ext_corp_id = ww2d3e2957190c6e4c）
--
-- 角色账号（tenant_code=demo，密码统一 123456）：
--   admin    / 123456  — 租户管理员（全菜单可见）
--   manager  / 123456  — 销售经理（客户/销售/运营/营销/风控可见，企业管理不可见）
--   sales    / 123456  — 销售（客户管理/线索/商机/智能名片/话术可见）
--
-- 平台超管（独立于租户，登录入口 /platform/login，密码统一 123456）：
--   master     / 123456  — 超级管理员（全部平台功能）
--   operator1  / 123456  — 运营（租户CRUD + 统计）
--   finance1   / 123456  — 财务（计费 + 租户只读 + 统计只读）
--   support1   / 123456  — 客服（租户只读 + 日志只读 + 统计只读）
--
-- 运行方式（scrm-dev-postgres 容器）：
--   docker exec -i scrm-dev-postgres psql -U root -d scrm < docs/mock-data.sql
--
-- 说明：
--   * 全部 INSERT 使用 ON CONFLICT (id) DO NOTHING，可重复执行（幂等）。
--   * id 采用 901… 高位号段，避免与既有 Snowflake 主键冲突。
--   * 关键外键取值取自现有 seed 数据（见文件尾部注释），运行前请核对。
--   * material.material_tag_list 存标签 id（jsonb 字符串数组）；welcome_msg.welcome_msg 为 jsonb。
-- =============================================================================

BEGIN;

-- ============================================================ CRM 角色（crm_role）
-- 角色由后端 seedTenantDefaults 自动创建（id 为 Snowflake 随机值），此处仅更新权限。
-- 不使用固定 ID 插入，而是通过 tenant_id + code 查找已有角色并更新 perms。
UPDATE crm_role SET perms = '["crm:user:set_password","crm:user:create","crm:risk:full","crm:marketing_ext:full","crm:group_sop:full","crm:smart_card:full"]'::jsonb
 WHERE tenant_id = 2087127714048774144 AND code = 'admin';
UPDATE crm_role SET perms = '["crm:user:create","crm:risk:full","crm:marketing_ext:full","crm:group_sop:full","crm:smart_card:full"]'::jsonb
 WHERE tenant_id = 2087127714048774144 AND code = 'manager';
UPDATE crm_role SET perms = '["crm:smart_card:full"]'::jsonb
 WHERE tenant_id = 2087127714048774144 AND code = 'sales';

-- ============================================================ CRM 用户（"user" 表）
-- 演示账号：admin/manager/sales/saler1/saler2，密码统一 123456（bcrypt 哈希）。
-- bcrypt hash of "123456"（与 SeedDemoAccount 生成的 admin 密码一致）:
--   $2a$10$XB0Gf8Gkq0kVFVlhdZZUBeCcDfEwUWRWUtFqSnb2Tb71mpBZOAD2.
-- role_id 通过子查询从 crm_role 动态查找，不依赖固定 ID。

-- admin：更新密码和角色（绑定到 admin 角色）
UPDATE "user" SET
  password = '$2a$10$XB0Gf8Gkq0kVFVlhdZZUBeCcDfEwUWRWUtFqSnb2Tb71mpBZOAD2.',
  role_id = (SELECT id FROM crm_role WHERE tenant_id = 2087127714048774144 AND code = 'admin' LIMIT 1),
  status = 1
 WHERE tenant_id = 2087127714048774144 AND username = 'admin';

-- saler1：更新密码和角色（绑定到 sales 角色）
UPDATE "user" SET
  password = '$2a$10$XB0Gf8Gkq0kVFVlhdZZUBeCcDfEwUWRWUtFqSnb2Tb71mpBZOAD2.',
  role_id = (SELECT id FROM crm_role WHERE tenant_id = 2087127714048774144 AND code = 'sales' LIMIT 1),
  status = 1
 WHERE tenant_id = 2087127714048774144 AND username = 'saler1';

-- saler2：更新密码和角色（绑定到 sales 角色）
UPDATE "user" SET
  password = '$2a$10$XB0Gf8Gkq0kVFVlhdZZUBeCcDfEwUWRWUtFqSnb2Tb71mpBZOAD2.',
  role_id = (SELECT id FROM crm_role WHERE tenant_id = 2087127714048774144 AND code = 'sales' LIMIT 1),
  status = 1
 WHERE tenant_id = 2087127714048774144 AND username = 'saler2';

-- manager：新增销售经理账号（如不存在则插入）
INSERT INTO "user"
  (id, created_at, updated_at, tenant_id, username, password, name, mobile, email,
   role_id, position, status)
SELECT 9010000000000000310, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'manager', '$2a$10$XB0Gf8Gkq0kVFVlhdZZUBeCcDfEwUWRWUtFqSnb2Tb71mpBZOAD2.',
   '张经理', '13800000001', 'manager@demo.com',
   (SELECT id FROM crm_role WHERE tenant_id = 2087127714048774144 AND code = 'manager' LIMIT 1),
   '销售经理', 1
 WHERE NOT EXISTS (
   SELECT 1 FROM "user" WHERE tenant_id = 2087127714048774144 AND username = 'manager' AND deleted_at IS NULL
 );

-- sales：新增独立销售账号（如不存在则插入）
INSERT INTO "user"
  (id, created_at, updated_at, tenant_id, username, password, name, mobile, email,
   role_id, position, status)
SELECT 9010000000000000311, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'sales', '$2a$10$XB0Gf8Gkq0kVFVlhdZZUBeCcDfEwUWRWUtFqSnb2Tb71mpBZOAD2.',
   '李销售', '13800000002', 'sales@demo.com',
   (SELECT id FROM crm_role WHERE tenant_id = 2087127714048774144 AND code = 'sales' LIMIT 1),
   '销售', 1
 WHERE NOT EXISTS (
   SELECT 1 FROM "user" WHERE tenant_id = 2087127714048774144 AND username = 'sales' AND deleted_at IS NULL
 );

COMMIT;

-- ============================================================ 平台超管（platform_admin）
-- 平台超管独立于租户，用于管理所有租户（创建/停用/删除/编辑）。
-- 登录入口：/platform/login，密码统一 123456。
-- bcrypt hash of "123456"（与 CRM 用户一致）:
--   $2a$10$XB0Gf8Gkq0kVFVlhdZZUBeCcDfEwUWRWUtFqSnb2Tb71mpBZOAD2.
--
-- 平台角色与权限：
--   super_admin  超级管理员  全部平台功能（租户CRUD/计费/统计/日志/设置）
--   operator     运营        租户CRUD + 统计
--   finance      财务        计费 + 租户只读 + 统计只读
--   support      客服        租户只读 + 日志只读 + 统计只读
--
-- 账号列表：
--   master     / 123456  — 超级管理员
--   operator1  / 123456  — 运营
--   finance1   / 123456  — 财务
--   support1   / 123456  — 客服
BEGIN;
INSERT INTO platform_admin
  (id, created_at, updated_at, username, password, name, avatar, mobile, email, role, status)
SELECT 9010000000000000400, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP,
   'master', '$2a$10$XB0Gf8Gkq0kVFVlhdZZUBeCcDfEwUWRWUtFqSnb2Tb71mpBZOAD2.',
   '超级管理员', '', '', '', 'super_admin', 1
 WHERE NOT EXISTS (
   SELECT 1 FROM platform_admin WHERE username = 'master' AND deleted_at IS NULL
);

INSERT INTO platform_admin
  (id, created_at, updated_at, username, password, name, avatar, mobile, email, role, status)
SELECT 9010000000000000401, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP,
   'operator1', '$2a$10$XB0Gf8Gkq0kVFVlhdZZUBeCcDfEwUWRWUtFqSnb2Tb71mpBZOAD2.',
   '运营小李', '', '', '', 'operator', 1
 WHERE NOT EXISTS (
   SELECT 1 FROM platform_admin WHERE username = 'operator1' AND deleted_at IS NULL
);

INSERT INTO platform_admin
  (id, created_at, updated_at, username, password, name, avatar, mobile, email, role, status)
SELECT 9010000000000000402, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP,
   'finance1', '$2a$10$XB0Gf8Gkq0kVFVlhdZZUBeCcDfEwUWRWUtFqSnb2Tb71mpBZOAD2.',
   '财务小张', '', '', '', 'finance', 1
 WHERE NOT EXISTS (
   SELECT 1 FROM platform_admin WHERE username = 'finance1' AND deleted_at IS NULL
);

INSERT INTO platform_admin
  (id, created_at, updated_at, username, password, name, avatar, mobile, email, role, status)
SELECT 9010000000000000403, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP,
   'support1', '$2a$10$XB0Gf8Gkq0kVFVlhdZZUBeCcDfEwUWRWUtFqSnb2Tb71mpBZOAD2.',
   '客服小王', '', '', '', 'support', 1
 WHERE NOT EXISTS (
   SELECT 1 FROM platform_admin WHERE username = 'support1' AND deleted_at IS NULL
);
COMMIT;

-- ============================================================ CRM 客户
INSERT INTO crm_customer
  (id, created_at, updated_at, tenant_id, name, mobile, email, company, industry,
   gender, birthday, level, address, source, owner_user_id, next_followup_at, remark, tags)
VALUES
  (9010000000000000001, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '王总', '13900000001', 'wang@bluesky.com', '蓝天科技', '互联网',
   1, '1985-03-12', 3, '北京市朝阳区', '展会', 2087390044095516672,
   CURRENT_TIMESTAMP + INTERVAL '7 day', '重点跟进中', '["VIP"]'),
  (9010000000000000002, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '李总', '13900000002', 'li@hongri.com', '红日集团', '制造业',
   2, '1990-07-08', 4, '上海市浦东新区', '官网', 2087390044095516672,
   CURRENT_TIMESTAMP + INTERVAL '3 day', '已有合作意向', NULL),
  (9010000000000000003, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '赵总', '13900000003', 'zhao@lvdi.com', '绿地贸易', '零售',
   1, '1988-11-21', 2, '广州市天河区', '转介绍', 2087129133212504064,
   NULL, '待首次拜访', NULL),
  (9010000000000000004, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '孙总', '13900000004', 'sun@changjiang.com', '长江电力', '能源',
   1, '1978-01-30', 5, '深圳市南山区', '行业峰会', 2087129133212504064,
   CURRENT_TIMESTAMP + INTERVAL '14 day', '大客户', NULL)
ON CONFLICT (id) DO NOTHING;

-- ============================================================ CRM 线索
INSERT INTO crm_lead
  (id, created_at, updated_at, tenant_id, customer_id, owner_user_id, title, company,
   name, mobile, source, status, remark)
VALUES
  (9010000000000000011, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000001, 2087390044095516672, '企业微信SCRM采购咨询', '未来科技',
   '钱经理', '13700000001', '广告投放', 'qualified', '已转化为客户'),
  (9010000000000000012, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   NULL, 2087390044095516672, '智能客服产品咨询', '云帆软件',
   '周经理', '13700000002', '百度推广', 'contacted', '已电话联系，待演示'),
  (9010000000000000013, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   NULL, 2087129333339525120, 'CRM系统升级需求', '星河网络',
   '吴经理', '13700000003', '线下活动', 'new', '新线索待跟进'),
  (9010000000000000014, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   NULL, 2087129333339525120, '物流SaaS意向', '宏远物流',
   '郑经理', '13700000004', '老客户转介绍', 'lost', '预算不足流失')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ CRM 商机
-- 阶段取值：S1=2087342710796062720 初步接洽 / S2=2087342710804451328 需求确认
--          S3=2087342710812839936 方案报价 / S4=2087342710817034240 商务谈判
--          S5=2087342710825422848 赢单     / S6=2087342710829617152 输单
INSERT INTO crm_opportunity
  (id, created_at, updated_at, tenant_id, customer_id, lead_id, owner_user_id, stage_id,
   title, product, amount, expected_close_date, status, remark)
VALUES
  (9010000000000000021, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000001, 9010000000000000011, 2087390044095516672, 2087342710812839936,
   '企业微信SCRM标准版采购', 'SCRM标准版', 120000, CURRENT_TIMESTAMP + INTERVAL '30 day',
   'open', '报价阶段，客户比较中'),
  (9010000000000000022, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000002, NULL, 2087390044095516672, 2087342710804451328,
   '客户管理系统实施', '实施服务', 80000, CURRENT_TIMESTAMP + INTERVAL '20 day',
   'open', '需求确认中'),
  (9010000000000000023, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000001, NULL, 2087390044095516672, 2087342710825422848,
   '年度续费合同', '续费', 50000, CURRENT_TIMESTAMP - INTERVAL '10 day',
   'won', '已签约赢单'),
  (9010000000000000024, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000003, NULL, 2087129133212504064, 2087342710829617152,
   '智能客服采购', '客服产品', 30000, CURRENT_TIMESTAMP - INTERVAL '5 day',
   'lost', '客户选择竞品')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ CRM 合同订单
INSERT INTO crm_order
  (id, created_at, updated_at, tenant_id, customer_id, opportunity_id, user_id, order_no,
   title, amount, paid_amount, status, pay_status, signed_at, remark)
VALUES
  (9010000000000000031, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000001, 9010000000000000023, 2087390044095516672, 'MO-20260801-001',
   '企业微信SCRM标准版合同', 50000, 50000, 'completed', 'paid',
   '2026-08-01 10:00:00+08', '全款已收清'),
  (9010000000000000032, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000002, 9010000000000000022, 2087390044095516672, 'MO-20260805-002',
   '实施服务合同', 30000, 10000, 'signed', 'partial',
   '2026-08-05 14:30:00+08', '预付首期款')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ CRM 跟进记录
INSERT INTO crm_followup
  (id, created_at, updated_at, tenant_id, customer_id, opportunity_id, user_id,
   type, content, next_followup_at)
VALUES
  (9010000000000000041, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000001, 9010000000000000021, 2087390044095516672,
   'call', '电话沟通了功能需求细节，约好下周演示', CURRENT_TIMESTAMP + INTERVAL '7 day'),
  (9010000000000000042, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000002, 9010000000000000022, 2087390044095516672,
   'meeting', '上门演示客户管理系统，客户评价较好', CURRENT_TIMESTAMP + INTERVAL '3 day'),
  (9010000000000000043, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000001, NULL, 2087390044095516672,
   'wechat', '微信发送了报价单和合同模板', NULL)
ON CONFLICT (id) DO NOTHING;

COMMIT;

-- ============================================================ 企业微信侧（wework 员工管理员 WangQiang 视角）
-- 以下表以 ext_corp_id 关联企微企业（demo = ww2d3e2957190c6e4c），
-- 由超级管理员 WangQiang（staff.id = 1）在员工管理后台查看。

BEGIN;

-- ---- 客户群发 ----
INSERT INTO mass_msg
  (id, ext_corp_id, ext_creator_id, send_type, ext_staff_ids, ext_department_ids, msg,
   ext_msg_id, mission_status, ext_customer_filter_enable, ext_customer_filter,
   delivered_num, success_num, un_delivered_num, failed_num, send_at, created_at, updated_at)
VALUES
  (9010000000000000101, 'ww2d3e2957190c6e4c', 'WangQiang', 1, '["WangQiang"]', '[]',
   '{"text":"新品上市优惠来啦，欢迎咨询~","attachments":[]}', 'MSG-20260801-001', 3,
   0, '{}', 1, 1, 0, 0, '2026-08-01 10:00:00', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  (9010000000000000102, 'ww2d3e2957190c6e4c', 'WangQiang', 2, '["WangQiang"]', '[]',
   '{"text":"春节客户答谢活动预告","attachments":[]}', 'MSG-20260810-002', 1,
   0, '{}', 0, 0, 0, 0, '2026-08-20 09:00:00', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO NOTHING;

-- ---- 素材库标签（material_tag_list 存放的是标签 id，非标签名）----
INSERT INTO material_lib_tag
  (id, ext_corp_id, ext_creator_id, name, created_at, updated_at)
VALUES
  (9010000000000000141, 'ww2d3e2957190c6e4c', 'WangQiang', '产品', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  (9010000000000000142, 'ww2d3e2957190c6e4c', 'WangQiang', '活动', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO NOTHING;

-- ---- 素材库（material_type 取值：link/poster/video/pdf/ppt/excel/word）----
INSERT INTO material
  (id, ext_corp_id, ext_creator_id, material_type, title, file_size, file_url, link,
   digest, material_tag_list, created_at, updated_at)
VALUES
  (9010000000000000111, 'ww2d3e2957190c6e4c', 'WangQiang', 'link', '企业微信SCRM产品介绍',
   0, '', 'https://example.com/product', '一套好用的企业微信客户运营工具',
   '["9010000000000000141"]', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  (9010000000000000112, 'ww2d3e2957190c6e4c', 'WangQiang', 'poster', '活动海报-夏季促销',
   204800, 'https://example.com/poster.png', '', '夏季促销活动主视觉',
   '["9010000000000000142"]', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO NOTHING;

-- ---- 渠道活码 ----
INSERT INTO contact_way
  (id, ext_corp_id, ext_creator_id, name, config_id, group_id, qr_code, remark,
   skip_verify, add_customer_count, created_at, updated_at)
VALUES
  (9010000000000000121, 'ww2d3e2957190c6e4c', 'WangQiang', '官网首页咨询活码', 'config-001', 0,
   'https://example.com/qrcode/001.png', '投放在官网首页的客户咨询入口', 1, 23,
   CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  (9010000000000000122, 'ww2d3e2957190c6e4c', 'WangQiang', '展会扫码添加活码', 'config-002', 0,
   'https://example.com/qrcode/002.png', '行业展会现场扫码添加', 1, 56,
   CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO NOTHING;

-- ---- 客户欢迎语（welcome_msg 表，客户欢迎语页面数据源）----
-- welcome_msg jsonb 结构：{"text":"...","attachments":[]}；enable_time_period_msg: 1=不启用 2=启用
INSERT INTO welcome_msg
  (id, ext_corp_id, ext_creator_id, name, welcome_msg, main_welcome_msg_id,
   enable_time_period_msg, created_at, updated_at)
VALUES
  (9010000000000000151, 'ww2d3e2957190c6e4c', 'WangQiang', '新人进群欢迎语',
   '{"text":"欢迎加入客户群，进群可领取新人福利~","attachments":[]}', NULL, 1,
   CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  (9010000000000000152, 'ww2d3e2957190c6e4c', 'WangQiang', '产品咨询欢迎语',
   '{"text":"感谢关注我们的产品，有任何问题随时咨询。","attachments":[]}', NULL, 1,
   CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO NOTHING;

-- ---- 客户群欢迎语（group_chat_welcome_msg 表，客户群欢迎语页面数据源）----
INSERT INTO group_chat_welcome_msg
  (id, ext_corp_id, ext_creator_id, content, attachment_type, attachment, created_at, updated_at)
VALUES
  (9010000000000000131, 'ww2d3e2957190c6e4c', 'WangQiang', '欢迎加入客户群，进群可领取新人福利~', 'none',
   '[]', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  (9010000000000000132, 'ww2d3e2957190c6e4c', 'WangQiang', '感谢关注我们的产品，有任何问题随时咨询。', 'none',
   '[]', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 渠道映射（CRM 核心 ↔ 企微渠道）
-- 统一会话：CRM 本地账号（admin/123456）经 ChannelLink 关联到企微员工 WangQiang（corp=ww2d3e2957190c6e4c），
-- 使密码登录的 JWT 同时携带 tenant/user 与 corp/staff 身份，企微渠道功能（渠道活码/素材库/员工管理等）共用同一会话。
INSERT INTO channel_link
  (id, created_at, updated_at, tenant_id, channel, entity_type, entity_id, ext_id, ext_corp_id)
VALUES
  (9010000000000000091, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'wecom', 'user', 2087390044095516672, 'WangQiang', 'ww2d3e2957190c6e4c'),
  (9010000000000000092, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'wecom', 'tenant', 2087127714048774144, 'ww2d3e2957190c6e4c', NULL)
ON CONFLICT (id) DO NOTHING;

-- 统一客户渠道维度：把 CRM 客户"王总"标为企微来源客户并回填渠道快照 + ChannelLink，
-- 使统一客户管理页能展示 添加渠道/账号类型/企微标签 等渠道信息。
UPDATE crm_customer
   SET source = 'wecom', ext_customer_id = 'wm_zt2MTJkBXQbZw001',
       ext_corp_id = 'ww2d3e2957190c6e4c', add_way = 1, account_type = '企业微信',
       out_flow_status = 0, channel_tags = '["VIP","重点客户"]'
 WHERE id = 9010000000000000001;
INSERT INTO channel_link
  (id, created_at, updated_at, tenant_id, channel, entity_type, entity_id, ext_id, ext_corp_id)
VALUES
  (9010000000000000093, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'wecom', 'customer', 9010000000000000001, 'wm_zt2MTJkBXQbZw001', 'ww2d3e2957190c6e4c')
ON CONFLICT (id) DO NOTHING;

-- 补充："李总"也标为企微来源客户（渠道回填 + ChannelLink），演示多渠道客户统一主档。
UPDATE crm_customer
   SET source = 'wecom', ext_customer_id = 'wm_zt2MTJkBXQbZw002',
       ext_corp_id = 'ww2d3e2957190c6e4c', add_way = 2, account_type = '企业微信',
       out_flow_status = 0, channel_tags = '["重点客户"]'
 WHERE id = 9010000000000000002;
INSERT INTO channel_link
  (id, created_at, updated_at, tenant_id, channel, entity_type, entity_id, ext_id, ext_corp_id)
VALUES
  (9010000000000000094, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'wecom', 'customer', 9010000000000000002, 'wm_zt2MTJkBXQbZw002', 'ww2d3e2957190c6e4c')
ON CONFLICT (id) DO NOTHING;

-- 补充：把"赵总"标为已流失，演示客户列表的 流失状态 筛选（out_flow_status=1）。
UPDATE crm_customer
   SET out_flow_status = 1, remark = '多次跟进无响应，标记已流失'
 WHERE id = 9010000000000000003;

-- ---- 企微互动（消息存档单聊，供客户 360 详情展示）----
-- 王总(ext=wm_zt2MTJkBXQbZw001) 与 员工 WangQiang 的会话；李总(ext=wm_zt2MTJkBXQbZw002) 同理。
INSERT INTO chat_msg
  (id, ext_corp_id, ext_creator_id, msg_id, action, "from", to_list, room_id,
   msg_time, msg_type, content_text, seq, session_id, session_type)
VALUES
  (9010000000000000161, 'ww2d3e2957190c6e4c', 'WangQiang', 'msg-90001', 'send',
   'wm_zt2MTJkBXQbZw001', '["WangQiang"]', '',
   1787000000000, 'text', '你好，你们SCRM的报价单我收到了，比较感兴趣，麻烦尽快联系我。', 1,
   'sess-zt2MTJkBXQbZw001-1', 'single'),
  (9010000000000000162, 'ww2d3e2957190c6e4c', 'WangQiang', 'msg-90002', 'send',
   'WangQiang', '["wm_zt2MTJkBXQbZw001"]', '',
   1787000060000, 'text', '好的王总，我安排销售明天上午给您电话详细沟通，稍后发您合同模板。', 2,
   'sess-zt2MTJkBXQbZw001-1', 'single'),
  (9010000000000000163, 'ww2d3e2957190c6e4c', 'WangQiang', 'msg-90003', 'send',
   'wm_zt2MTJkBXQbZw001', '["WangQiang"]', '',
   1787000120000, 'text', '嗯，合同模板也发我看下，还有实施周期大概多久？', 3,
   'sess-zt2MTJkBXQbZw001-1', 'single'),
  (9010000000000000164, 'ww2d3e2957190c6e4c', 'WangQiang', 'msg-90004', 'send',
   'wm_zt2MTJkBXQbZw002', '["WangQiang"]', '',
   1787001000000, 'text', '李总，上次聊的实施方案您这边评估得怎么样了？', 1,
   'sess-zt2MTJkBXQbZw002-1', 'single')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 营销活动
INSERT INTO crm_activity
  (id, created_at, updated_at, tenant_id, owner_user_id, name, activity_type,
   status, channel, start_at, end_at, cost, reach_count, participate_count,
   conversion_count, reward_desc, remark)
VALUES
  (9010000000000000201, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   2087390044095516672, '老客回馈·优惠券', 'coupon', 'active', 'wecom',
   CURRENT_TIMESTAMP - INTERVAL '5 day', CURRENT_TIMESTAMP + INTERVAL '25 day',
   5000.00, 320, 210, 58, '新老客户各发满100减20优惠券，转介绍再送一张',
   '618 预热，覆盖企微存量客户'),
  (9010000000000000202, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   2087390044095516672, '新品上市·抽奖', 'lottery', 'active', 'wecom',
   CURRENT_TIMESTAMP - INTERVAL '2 day', CURRENT_TIMESTAMP + INTERVAL '13 day',
   3000.00, 150, 96, 22, '关注公众号并留资参与抽奖，奖品为新品蓝牙耳机',
   '配合新品发布做拉新'),
  (9010000000000000203, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   2087390044095516672, '夏季秒杀·专场', 'seckill', 'ended', 'wecom',
   CURRENT_TIMESTAMP - INTERVAL '30 day', CURRENT_TIMESTAMP - INTERVAL '16 day',
   8000.00, 512, 388, 74, '精选 SKU 限时秒杀，每晚 8 点开抢',
   '复盘：转化率高于均值，可复制')
ON CONFLICT (id) DO NOTHING;

COMMIT;

-- ============================================================ 小红书渠道（demo）
-- 小红书客户：留资/私信承接进入统一客户主档，source=xiaohongshu，经 ChannelLink(channel=xiaohongshu) 关联。

BEGIN;

-- ---- 小红书客户（渠道快照列 + 标签快照）----
INSERT INTO crm_customer
  (id, created_at, updated_at, tenant_id, name, mobile, email, company, industry,
   gender, level, address, source, owner_user_id, next_followup_at, remark, tags,
   ext_customer_id, ext_corp_id, add_way, account_type, out_flow_status, channel_tags)
VALUES
  (9010000000000000005, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '周小姐', '13900000005', 'zhou@xhs.com', '小红书留资', '消费品',
   2, 3, '上海市静安区', 'xiaohongshu', 2087390044095516672,
   CURRENT_TIMESTAMP + INTERVAL '5 day', '小红书「新品测评」笔记留资，意向较高', NULL,
   'xhs_user_20260001', 'xhs_corp_demo01', 1, '小红书', 0, '["种草","高潜"]'),
  (9010000000000000006, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '陈女士', '13900000006', 'chen@xhs.com', '小红书私信', '零售',
   2, 2, '杭州市西湖区', 'xiaohongshu', 2087390044095516672,
   NULL, '小红书私信咨询优惠券，待承接', NULL,
   'xhs_user_20260002', 'xhs_corp_demo01', 2, '小红书', 0, '["高互动"]')
ON CONFLICT (id) DO NOTHING;

INSERT INTO channel_link
  (id, created_at, updated_at, tenant_id, channel, entity_type, entity_id, ext_id, ext_corp_id)
VALUES
  (9010000000000000095, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'xiaohongshu', 'customer', 9010000000000000005, 'xhs_user_20260001', 'xhs_corp_demo01'),
  (9010000000000000096, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'xiaohongshu', 'customer', 9010000000000000006, 'xhs_user_20260002', 'xhs_corp_demo01')
ON CONFLICT (id) DO NOTHING;

-- ---- 小红书互动记录（留资/评论/私信，供 360 渠道互动时间线）----
INSERT INTO crm_channel_interaction
  (id, created_at, updated_at, tenant_id, customer_id, channel, action_type, content, happened_at, ext_ref)
VALUES
  (9010000000000000171, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000005, 'xiaohongshu', 'lead', '通过「新品测评」笔记留资表单提交：姓名周小姐，手机 13900000005', CURRENT_TIMESTAMP - INTERVAL '2 day', 'note-20260801'),
  (9010000000000000172, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000005, 'xiaohongshu', 'comment', '笔记评论：「这个测评写得好，优惠券怎么领？」', CURRENT_TIMESTAMP - INTERVAL '1 day', 'note-20260801'),
  (9010000000000000173, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000005, 'xiaohongshu', 'dm', '私信：「已私信索取优惠券，等企业号回复」', CURRENT_TIMESTAMP - INTERVAL '12 hour', 'dm-20260810'),
  (9010000000000000174, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000006, 'xiaohongshu', 'comment', '评论区留言：「已收藏，求私信链接」', CURRENT_TIMESTAMP - INTERVAL '3 day', 'note-20260728'),
  (9010000000000000175, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000006, 'xiaohongshu', 'dm', '私信咨询：「你们 618 有优惠吗？企微怎么加？」', CURRENT_TIMESTAMP - INTERVAL '6 hour', 'dm-20260812')
ON CONFLICT (id) DO NOTHING;

-- ---- 渠道标签目录（幂等：同名目录已存在则跳过，避免与启动 seed 重复）----
INSERT INTO crm_channel_tag (id, created_at, updated_at, tenant_id, channel, name, category, sort)
SELECT t.id, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, t.tenant_id, t.channel, t.name, t.category, t.sort
FROM (VALUES
  (9010000000000000181, 2087127714048774144, 'wecom', '重点客户', '客户价值', 0),
  (9010000000000000182, 2087127714048774144, 'wecom', 'VIP', '客户价值', 1),
  (9010000000000000183, 2087127714048774144, 'wecom', '高意向', '意向', 2),
  (9010000000000000184, 2087127714048774144, 'wecom', '已成交', '成交', 3),
  (9010000000000000185, 2087127714048774144, 'xiaohongshu', '种草', '互动', 4),
  (9010000000000000186, 2087127714048774144, 'xiaohongshu', '高互动', '互动', 5),
  (9010000000000000187, 2087127714048774144, 'xiaohongshu', '高潜', '意向', 6),
  (9010000000000000188, 2087127714048774144, 'xiaohongshu', '已承接', '承接', 7)
) AS t(id, tenant_id, channel, name, category, sort)
WHERE NOT EXISTS (SELECT 1 FROM crm_channel_tag WHERE tenant_id = t.tenant_id AND channel = t.channel AND name = t.name);

-- ---- 营销活动多渠道：存量单渠道活动回填 channels；新增小红书跨渠道活动 ----
UPDATE crm_activity SET channels = '["wecom"]'::jsonb WHERE channels IS NULL AND channel = 'wecom';

INSERT INTO crm_activity
  (id, created_at, updated_at, tenant_id, owner_user_id, name, activity_type,
   status, channel, channels, start_at, end_at, cost, reach_count, participate_count,
   conversion_count, reward_desc, remark)
VALUES
  (9010000000000000204, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   2087390044095516672, '小红书种草·留资抽奖', 'lottery', 'active', 'xiaohongshu', '["xiaohongshu","wecom"]',
   CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP + INTERVAL '14 day',
   2000.00, 86, 41, 9, '小红书笔记评论+留资可抽新品盲盒，企微承接后加赠优惠券',
   '跨渠道活动：小红书种草引流，企微承接转化')
ON CONFLICT (id) DO NOTHING;

COMMIT;

-- =============================================================================
-- 关键外键取值说明（demo 租户 seed 后稳定存在，如需核对请执行对应 SELECT）：
--   租户      tenant.id                        = 2087127714048774144（demo）
--   CRM 角色  crm_role.id admin/manager/sales     = 由 seedTenantDefaults 自动生成（Snowflake 随机值）
--            查找方式: SELECT id, code FROM crm_role WHERE tenant_id = 2087127714048774144
--   本地用户  "user".id admin(密码登录)         = 2087390044095516672
--   本地用户  "user".id saler1 / 销售小王       = 2087129133212504064
--   本地用户  "user".id saler2 / 销售小李       = 2087129333339525120
--   本地用户  "user".id manager / 张经理        = 9010000000000000310
--   本地用户  "user".id sales / 李销售          = 9010000000000000311
--   登录账号  tenant_code=demo, 密码统一 123456
--             admin    → 租户管理员（全菜单）
--             manager  → 销售经理（无企业管理）
--             sales    → 销售（仅客户/线索/商机/名片/话术）
--             saler1   → 销售（同 sales 角色）
--             saler2   → 销售（同 sales 角色）
--   销售阶段  crm_pipeline_stage.id             = 2087342710796062720(S1) …
--                                                2087342710829617152(S6)
--   营销活动  crm_activity.owner_user_id        = 2087390044095516672(admin)
--             状态 draft/active/ended/off；渠道 wecom/xiaohongshu/general（channels 多选，channel 为主渠道）
--   小红书客户  crm_customer.source             = xiaohongshu
--             渠道快照 ext_customer_id/ext_corp_id/add_way/account_type/channel_tags，经 ChannelLink(channel=xiaohongshu) 关联
--   渠道互动    crm_channel_interaction         = 小红书留资/评论/私信（企微互动仍走 chat_msg）
-- =============================================================================

-- =============================================================================
-- 以下为按 biz-func-flows.md 功能流程清单补充的演示数据
-- 覆盖：报价/回款/优惠券/秒杀/抽奖/拼团/砍价/满减/红包/产品/仓库/库存/
--       知识库/公告/工作报告/工单/售后/SOP/审批/自动分配/通话/签到/待办/
--       发票/财务/供应商/采购/项目/敏感词/AI分析/裂变/落地页/智能名片
-- =============================================================================

BEGIN;

-- ============================================================ 报价单（crm_quote）
INSERT INTO crm_quote
  (id, created_at, updated_at, tenant_id, quote_no, customer_id, opportunity_id,
   subject, amount, discount, final_amount, items, valid_until, status, owner_user_id,
   remark, tax_rate, tax_amount)
VALUES
  (9010000000000000301, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'QT-20260810-001', 9010000000000000001, 9010000000000000021,
   '企业微信SCRM标准版报价', 120000, 5000, 115000,
   '["{\"name\":\"SCRM标准版\",\"spec\":\"1年\",\"qty\":1,\"price\":120000}"]',
   CURRENT_TIMESTAMP + INTERVAL '30 day', 'sent', 2087390044095516672,
   '含实施服务，有效期30天', 0.06, 6900),
  (9010000000000000302, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'QT-20260812-002', 9010000000000000002, 9010000000000000022,
   '客户管理系统实施报价', 80000, 0, 80000,
   '["{\"name\":\"实施服务\",\"spec\":\"定制开发\",\"qty\":1,\"price\":80000}"]',
   CURRENT_TIMESTAMP + INTERVAL '15 day', 'accepted', 2087390044095516672,
   '客户已接受报价', 0.06, 4800),
  (9010000000000000303, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'QT-20260815-003', 9010000000000000003, NULL,
   '智能客服产品报价', 30000, 2000, 28000,
   '["{\"name\":\"智能客服\",\"spec\":\"SaaS版\",\"qty\":1,\"price\":30000}"]',
   CURRENT_TIMESTAMP - INTERVAL '5 day', 'void', 2087129133212504064,
   '客户选择竞品，已作废', 0.06, 1680)
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 回款计划（crm_receivables_plan）
INSERT INTO crm_receivables_plan
  (id, created_at, updated_at, tenant_id, order_id, customer_id, period_no,
   amount, plan_date, status, remark, contract_ratio)
VALUES
  (9010000000000000321, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000032, 9010000000000000002, 1, 10000,
   CURRENT_TIMESTAMP - INTERVAL '3 day', 'received', '首期款已到账', 0.33),
  (9010000000000000322, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000032, 9010000000000000002, 2, 10000,
   CURRENT_TIMESTAMP + INTERVAL '30 day', 'pending', '二期款待收', 0.33),
  (9010000000000000323, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000032, 9010000000000000002, 3, 10000,
   CURRENT_TIMESTAMP + INTERVAL '60 day', 'pending', '尾款待收', 0.34)
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 回款记录（crm_receivables）
INSERT INTO crm_receivables
  (id, created_at, updated_at, tenant_id, order_id, customer_id, plan_id,
   amount, received_at, pay_method, pay_account, voucher_no, status, remark,
   receivables_no, approve_status)
VALUES
  (9010000000000000331, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000031, 9010000000000000001, NULL,
   50000, CURRENT_TIMESTAMP - INTERVAL '20 day', 'transfer', '招行6225xxxx1234',
   'V-20260801-001', 'confirmed', '全款到账', 'RC-20260801-001', 'none'),
  (9010000000000000332, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000032, 9010000000000000002, 9010000000000000321,
   10000, CURRENT_TIMESTAMP - INTERVAL '3 day', 'wechat', '微信商户号',
   'V-20260819-002', 'confirmed', '首期款到账', 'RC-20260819-002', 'none')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 优惠券（crm_coupon）
INSERT INTO crm_coupon
  (id, created_at, updated_at, tenant_id, activity_id, name, coupon_type,
   face_value, threshold, total_count, issued_count, used_count,
   start_at, end_at, status, remark)
VALUES
  (9010000000000000401, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000201, '满100减20券', 'cash', 20, 100, 500, 320, 58,
   CURRENT_TIMESTAMP - INTERVAL '5 day', CURRENT_TIMESTAMP + INTERVAL '25 day',
   'active', '618老客回馈活动券'),
  (9010000000000000402, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000201, '8.5折折扣券', 'discount', 0.85, 200, 200, 86, 12,
   CURRENT_TIMESTAMP - INTERVAL '5 day', CURRENT_TIMESTAMP + INTERVAL '25 day',
   'active', '高客单价客户专享'),
  (9010000000000000403, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   NULL, '新品体验兑换券', 'gift', 0, 0, 100, 30, 8,
   CURRENT_TIMESTAMP - INTERVAL '10 day', CURRENT_TIMESTAMP + INTERVAL '80 day',
   'active', '兑换新品试用装')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 秒杀活动（crm_seckill）+ 记录
INSERT INTO crm_seckill
  (id, created_at, updated_at, tenant_id, activity_id, name, product_name,
   original_price, seckill_price, total_stock, sold_count, limit_per_user,
   start_at, end_at, status, remark)
VALUES
  (9010000000000000501, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000203, '夏季秒杀·SCRM基础版', 'SCRM基础版年卡',
   9800, 1999, 100, 74, 1,
   CURRENT_TIMESTAMP - INTERVAL '30 day', CURRENT_TIMESTAMP - INTERVAL '16 day',
   'ended', '已结束，转化率74%'),
  (9010000000000000502, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   NULL, '秋季特惠·智能客服', '智能客服月卡',
   599, 99, 200, 0, 1,
   CURRENT_TIMESTAMP + INTERVAL '2 day', CURRENT_TIMESTAMP + INTERVAL '5 day',
   'active', '即将开始')
ON CONFLICT (id) DO NOTHING;

INSERT INTO crm_seckill_record
  (id, created_at, updated_at, tenant_id, seckill_id, customer_id, price,
   status, grabbed_at, remark)
VALUES
  (9010000000000000511, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000501, 9010000000000000001, 1999, 'paid',
   CURRENT_TIMESTAMP - INTERVAL '25 day', '已支付'),
  (9010000000000000512, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000501, 9010000000000000002, 1999, 'paid',
   CURRENT_TIMESTAMP - INTERVAL '24 day', '已支付'),
  (9010000000000000513, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000501, 9010000000000000005, 1999, 'cancelled',
   CURRENT_TIMESTAMP - INTERVAL '23 day', '超时未支付取消')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 抽奖活动（crm_lottery）+ 记录
INSERT INTO crm_lottery
  (id, created_at, updated_at, tenant_id, activity_id, name, prizes,
   daily_limit, total_limit, drawn_count, start_at, end_at, status, remark)
VALUES
  (9010000000000000521, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000202, '新品上市抽奖', '["{\"name\":\"蓝牙耳机\",\"type\":\"gift\",\"weight\":10,\"count\":5,\"issued\":3}","{\"name\":\"满200减50券\",\"type\":\"coupon\",\"weight\":30,\"count\":50,\"issued\":18}","{\"name\":\"谢谢参与\",\"type\":\"thanks\",\"weight\":60,\"count\":0,\"issued\":0}"]',
   1, 0, 96, CURRENT_TIMESTAMP - INTERVAL '2 day', CURRENT_TIMESTAMP + INTERVAL '13 day',
   'active', '新品发布拉新活动')
ON CONFLICT (id) DO NOTHING;

INSERT INTO crm_lottery_record
  (id, created_at, updated_at, tenant_id, lottery_id, customer_id,
   prize_name, prize_type, is_win, drawn_at, remark)
VALUES
  (9010000000000000531, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000521, 9010000000000000001, '蓝牙耳机', 'gift', true,
   CURRENT_TIMESTAMP - INTERVAL '1 day', '一等奖'),
  (9010000000000000532, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000521, 9010000000000000005, '满200减50券', 'coupon', true,
   CURRENT_TIMESTAMP - INTERVAL '1 day', '二等奖'),
  (9010000000000000533, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000521, 9010000000000000006, '谢谢参与', 'thanks', false,
   CURRENT_TIMESTAMP - INTERVAL '12 hour', '未中奖')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 拼团活动（crm_group_buy）+ 记录
INSERT INTO crm_group_buy
  (id, created_at, updated_at, tenant_id, name, product_name, original_price,
   group_price, group_size, time_limit, max_groups, opened_groups,
   start_at, end_at, status, remark)
VALUES
  (9010000000000000541, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '三人成团·SCRM标准版', 'SCRM标准版年卡', 9800, 6800, 3, 24, 50, 12,
   CURRENT_TIMESTAMP - INTERVAL '3 day', CURRENT_TIMESTAMP + INTERVAL '11 day',
   'active', '3人拼团享7折'),
  (9010000000000000542, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '五人成团·智能客服', '智能客服年卡', 5999, 3999, 5, 48, 20, 3,
   CURRENT_TIMESTAMP - INTERVAL '10 day', CURRENT_TIMESTAMP - INTERVAL '2 day',
   'ended', '已结束')
ON CONFLICT (id) DO NOTHING;

INSERT INTO crm_group_buy_record
  (id, created_at, updated_at, tenant_id, group_buy_id, leader_id, member_ids,
   current_size, status, expire_at, success_at)
VALUES
  (9010000000000000551, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000541, 9010000000000000001, '["9010000000000000001","9010000000000000002","9010000000000000005"]',
   3, 'success', CURRENT_TIMESTAMP - INTERVAL '2 day', CURRENT_TIMESTAMP - INTERVAL '1 day'),
  (9010000000000000552, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000541, 9010000000000000003, '["9010000000000000003"]',
   1, 'open', CURRENT_TIMESTAMP + INTERVAL '12 hour', NULL),
  (9010000000000000553, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000542, 9010000000000000006, '["9010000000000000006"]',
   1, 'failed', CURRENT_TIMESTAMP - INTERVAL '2 day', NULL)
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 砍价活动（crm_bargain）+ 记录
INSERT INTO crm_bargain
  (id, created_at, updated_at, tenant_id, name, product_name, original_price,
   floor_price, max_helpers, per_cut_min, per_cut_max, time_limit,
   start_at, end_at, status, remark)
VALUES
  (9010000000000000561, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '砍价拿SCRM', 'SCRM基础版', 9800, 4900, 10, 50, 500, 48,
   CURRENT_TIMESTAMP - INTERVAL '2 day', CURRENT_TIMESTAMP + INTERVAL '12 day',
   'active', '帮砍最多10人，底价5折')
ON CONFLICT (id) DO NOTHING;

INSERT INTO crm_bargain_record
  (id, created_at, updated_at, tenant_id, bargain_id, customer_id,
   current_price, helper_ids, helper_cnt, status, expire_at)
VALUES
  (9010000000000000571, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000561, 9010000000000000001, 6200,
   '["9010000000000000002","9010000000000000005"]', 2, 'active',
   CURRENT_TIMESTAMP + INTERVAL '36 hour'),
  (9010000000000000572, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000561, 9010000000000000003, 4900,
   '["9010000000000000001","9010000000000000002","9010000000000000005","9010000000000000006"]',
   4, 'success', CURRENT_TIMESTAMP - INTERVAL '1 day')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 满减活动（crm_full_reduction）
INSERT INTO crm_full_reduction
  (id, created_at, updated_at, tenant_id, name, rule_type, rules,
   start_at, end_at, status, remark)
VALUES
  (9010000000000000581, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '满额阶梯减', 'full_amount',
   '["{\"threshold\":1000,\"discount\":50}","{\"threshold\":5000,\"discount\":300}","{\"threshold\":10000,\"discount\":800}"]',
   CURRENT_TIMESTAMP - INTERVAL '5 day', CURRENT_TIMESTAMP + INTERVAL '25 day',
   'active', '满1000减50，满5000减300，满10000减800'),
  (9010000000000000582, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '满件优惠', 'full_count',
   '["{\"threshold\":3,\"discount\":100}","{\"threshold\":5,\"discount\":200}"]',
   CURRENT_TIMESTAMP - INTERVAL '3 day', CURRENT_TIMESTAMP + INTERVAL '27 day',
   'active', '买3件减100，买5件减200')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 红包活动（crm_red_packet）+ 记录
INSERT INTO crm_red_packet
  (id, created_at, updated_at, tenant_id, name, total_amount, per_amount,
   total_count, issued_count, invite_required, start_at, end_at, status, remark)
VALUES
  (9010000000000000591, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '新人红包雨', 5000, 10, 500, 186, 0,
   CURRENT_TIMESTAMP - INTERVAL '5 day', CURRENT_TIMESTAMP + INTERVAL '25 day',
   'active', '无门槛领取10元红包'),
  (9010000000000000592, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '邀请有礼红包', 10000, 20, 500, 42, 3,
   CURRENT_TIMESTAMP - INTERVAL '2 day', CURRENT_TIMESTAMP + INTERVAL '28 day',
   'active', '邀请3人可领20元红包')
ON CONFLICT (id) DO NOTHING;

INSERT INTO crm_red_packet_record
  (id, created_at, updated_at, tenant_id, red_packet_id, customer_id,
   amount, invite_cnt, status, issued_at)
VALUES
  (9010000000000000601, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000591, 9010000000000000001, 10, 0, 'received',
   CURRENT_TIMESTAMP - INTERVAL '4 day'),
  (9010000000000000602, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000591, 9010000000000000005, 10, 0, 'received',
   CURRENT_TIMESTAMP - INTERVAL '3 day'),
  (9010000000000000603, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000592, 9010000000000000002, 20, 3, 'received',
   CURRENT_TIMESTAMP - INTERVAL '1 day')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 产品（crm_product）
INSERT INTO crm_product
  (id, created_at, updated_at, tenant_id, name, category, spec, unit,
   price, cost, sku, status, product_no, stock_qty, min_stock, remark)
VALUES
  (9010000000000000701, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'SCRM标准版年卡', '软件', '1年/10坐席', '套', 9800, 2000,
   'SCRM-STD-1Y', 'active', 'P001', 85, 10, '主力产品'),
  (9010000000000000702, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'SCRM企业版年卡', '软件', '1年/不限坐席', '套', 29800, 8000,
   'SCRM-ENT-1Y', 'active', 'P002', 12, 5, '高端产品'),
  (9010000000000000703, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '智能客服月卡', '软件', '1月', '个', 599, 50,
   'AI-CS-1M', 'active', 'P003', 8, 20, '库存预警中'),
  (9010000000000000704, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '实施服务', '服务', '定制', '次', 30000, 15000,
   'IMPL-CUSTOM', 'active', 'P004', 0, 0, '按需提供'),
  (9010000000000000705, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '培训服务', '服务', '1天/现场', '次', 5000, 2000,
   'TRAIN-1D', 'active', 'P005', 0, 0, '按需提供')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 仓库（crm_warehouse）
INSERT INTO crm_warehouse
  (id, created_at, updated_at, tenant_id, name, address, manager, status, warehouse_no)
VALUES
  (9010000000000000711, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '北京总仓', '北京市朝阳区科技园', 2087390044095516672, 'active', 'WH001'),
  (9010000000000000712, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '上海分仓', '上海市浦东新区张江高科', 2087129133212504064, 'active', 'WH002')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 库存流水（crm_stock_record）
INSERT INTO crm_stock_record
  (id, created_at, updated_at, tenant_id, warehouse_id, product_id, type,
   qty, order_no, occur_at, operator_id, remark)
VALUES
  (9010000000000000721, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000711, 9010000000000000701, 'instock', 100, 'PO-20260801-001',
   CURRENT_TIMESTAMP - INTERVAL '20 day', 2087390044095516672, '采购入库100套'),
  (9010000000000000722, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000711, 9010000000000000701, 'outstock', -15, 'MO-20260801-001',
   CURRENT_TIMESTAMP - INTERVAL '15 day', 2087390044095516672, '订单出库15套'),
  (9010000000000000723, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000711, 9010000000000000702, 'instock', 20, 'PO-20260805-002',
   CURRENT_TIMESTAMP - INTERVAL '18 day', 2087390044095516672, '采购入库20套'),
  (9010000000000000724, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000711, 9010000000000000703, 'instock', 50, 'PO-20260810-003',
   CURRENT_TIMESTAMP - INTERVAL '12 day', 2087129133212504064, '采购入库50个'),
  (9010000000000000725, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000711, 9010000000000000703, 'outstock', -42, 'MO-20260815-002',
   CURRENT_TIMESTAMP - INTERVAL '7 day', 2087129133212504064, '秒杀活动出库42个'),
  (9010000000000000726, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000711, 9010000000000000701, 'transfer', -10, 'TR-20260818-001',
   CURRENT_TIMESTAMP - INTERVAL '4 day', 2087390044095516672, '调拨至上海分仓10套'),
  (9010000000000000727, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000712, 9010000000000000701, 'transfer', 10, 'TR-20260818-001',
   CURRENT_TIMESTAMP - INTERVAL '4 day', 2087129133212504064, '从北京总仓调入10套'),
  (9010000000000000728, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000711, 9010000000000000701, 'check', 0, 'CK-20260820-001',
   CURRENT_TIMESTAMP - INTERVAL '2 day', 2087390044095516672, '盘点无差异')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 知识库分类（crm_knowledge_type）
INSERT INTO crm_knowledge_type
  (id, created_at, updated_at, tenant_id, name, sort)
VALUES
  (9010000000000000801, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '产品知识', 0),
  (9010000000000000802, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '销售话术', 1),
  (9010000000000000803, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '常见问题', 2),
  (9010000000000000804, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '公司制度', 3)
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 知识库（crm_knowledge）
INSERT INTO crm_knowledge
  (id, created_at, updated_at, tenant_id, knowledge_type_id, title, content,
   visibility, author_id, status, public_type)
VALUES
  (9010000000000000811, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000801, 'SCRM标准版功能清单',
   'SCRM标准版包含：客户管理、线索管理、商机管理、合同订单、营销活动等核心模块...',
   'company', 2087390044095516672, 'published', 'company'),
  (9010000000000000812, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000801, 'SCRM企业版与标准版对比',
   '企业版额外支持：不限坐席、高级数据分析、API开放平台、专属客户成功经理...',
   'company', 2087390044095516672, 'published', 'company'),
  (9010000000000000813, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000802, '客户说"太贵了"怎么应对',
   '话术1：价值拆解法——将年费拆解到每天，强调日均成本低于一杯咖啡...',
   'company', 2087390044095516672, 'published', 'company'),
  (9010000000000000814, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000803, '如何申请发票',
   '客户在订单详情页点击"申请发票"，填写抬头和税号后提交，财务审核后开具...',
   'company', 2087390044095516672, 'published', 'company'),
  (9010000000000000815, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000804, '销售提成制度2026版',
   '基础提成5%，超额完成目标部分提成10%，大客户额外奖励2%...',
   'department', 2087390044095516672, 'draft', 'department')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 公告（crm_notice）
INSERT INTO crm_notice
  (id, created_at, updated_at, tenant_id, title, content, type,
   publish_at, expire_at, status)
VALUES
  (9010000000000000821, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '618营销活动启动通知', '各位同事，618营销活动正式启动，请各销售做好客户触达准备...',
   'info', CURRENT_TIMESTAMP - INTERVAL '5 day', CURRENT_TIMESTAMP + INTERVAL '25 day', 'published'),
  (9010000000000000822, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '系统维护通知', '本周六凌晨2:00-4:00系统维护升级，期间无法使用...',
   'warning', CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP + INTERVAL '1 day', 'published'),
  (9010000000000000823, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '季度冲刺动员', 'Q3冲刺目标已下达，请各团队全力以赴完成季度目标！',
   'urgent', CURRENT_TIMESTAMP + INTERVAL '1 day', NULL, 'draft')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 工作报告（crm_work_report）
INSERT INTO crm_work_report
  (id, created_at, updated_at, tenant_id, user_id, type, report_date,
   content, plan, status, reviewer_id)
VALUES
  (9010000000000000831, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   2087390044095516672, 'daily', CURRENT_TIMESTAMP - INTERVAL '1 day',
   '今日拜访蓝天科技王总，沟通了SCRM采购需求，客户预算12万，下周安排演示。',
   '明日计划：1.准备演示PPT 2.联系李总确认实施方案评估 3.跟进小红书留资客户',
   'reviewed', 9010000000000000310),
  (9010000000000000832, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   2087129133212504064, 'weekly', CURRENT_TIMESTAMP - INTERVAL '2 day',
   '本周工作总结：1.新增线索3条 2.拜访客户5家 3.签约蓝天科技年度续费5万 4.跟进绿地贸易智能客服需求',
   '下周计划：1.完成绿地贸易报价 2.跟进长江电力大客户 3.开发3个新线索',
   'submitted', NULL),
  (9010000000000000833, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   2087129333339525120, 'daily', CURRENT_TIMESTAMP,
   '今日电话联系4个线索，其中2个有意向，已加微信跟进。',
   '明日计划：继续跟进2个意向线索，争取安排演示',
   'submitted', NULL)
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 工单类型（crm_workorder_type）
INSERT INTO crm_workorder_type
  (id, created_at, updated_at, tenant_id, name, sort, status, remark)
VALUES
  (9010000000000000841, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '实施部署', 0, 1, '系统部署实施类工单'),
  (9010000000000000842, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '技术支持', 1, 1, '技术问题咨询'),
  (9010000000000000843, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '培训需求', 2, 1, '客户培训申请'),
  (9010000000000000844, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '投诉处理', 3, 1, '客户投诉')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 工单（crm_workorder）
INSERT INTO crm_workorder
  (id, created_at, updated_at, tenant_id, no, title, type, customer_id, order_id,
   priority, assignee_id, description, status, workorder_type_id, source, channel)
VALUES
  (9010000000000000851, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'WO-20260805-001', '蓝天科技SCRM系统部署', '实施部署', 9010000000000000001,
   9010000000000000031, 'high', 2087390044095516672,
   '客户已签约，需要安排技术团队上门部署SCRM标准版系统，预计3个工作日',
   'done', 9010000000000000841, 'manual', 'wecom'),
  (9010000000000000852, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'WO-20260812-002', '红日集团数据迁移咨询', '技术支持', 9010000000000000002,
   9010000000000000032, 'medium', 2087129133212504064,
   '客户咨询如何将现有CRM数据迁移到我们系统，需要提供迁移方案',
   'processing', 9010000000000000842, 'manual', 'wecom'),
  (9010000000000000853, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'WO-20260818-003', '绿地贸易操作培训', '培训需求', 9010000000000000003,
   NULL, 'low', 2087129333339525120,
   '客户申请安排一次线上操作培训，参加人数约10人',
   'open', 9010000000000000843, 'manual', 'phone')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 售后（crm_aftersales）
INSERT INTO crm_aftersales
  (id, created_at, updated_at, tenant_id, no, order_id, customer_id, type,
   reason, amount, status, handler_id, result, done_at, remark)
VALUES
  (9010000000000000861, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'AS-20260810-001', 9010000000000000031, 9010000000000000001, 'repair',
   '客户反馈系统偶尔卡顿，需要排查优化', 0, 'done',
   2087390044095516672, '已排查为服务器内存不足，扩容后解决',
   CURRENT_TIMESTAMP - INTERVAL '5 day', '免费维修'),
  (9010000000000000862, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'AS-20260815-002', 9010000000000000032, 9010000000000000002, 'refund',
   '客户对实施服务不满意，要求退还部分费用', 5000, 'processing',
   2087129133212504064, '', NULL, '待主管审批')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ SOP规则（crm_sop_rule）
INSERT INTO crm_sop_rule
  (id, created_at, updated_at, tenant_id, name, trigger_type, trigger_cond,
   action_type, action_content, followup_type, delay_hours, status, remark)
VALUES
  (9010000000000000901, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '商机进入方案报价阶段→创建跟进', 'stage_change', '2087342710812839936',
   'create_followup', '商机已进入方案报价阶段，请尽快准备报价单并发送给客户',
   'call', 0, 'active', '方案报价阶段自动提醒'),
  (9010000000000000902, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '打VIP标签→发送关怀消息', 'tag_added', 'VIP',
   'send_message', '尊敬的VIP客户，感谢您对我们的支持，专属客服已为您就位！',
   '', 0, 'active', 'VIP客户自动关怀'),
  (9010000000000000903, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '新客户→24小时后创建跟进', 'new_customer', '',
   'create_followup', '新客户已录入系统，请在24小时内完成首次联系',
   'call', 24, 'active', '新客户首联提醒'),
  (9010000000000000904, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '公海认领→立即创建跟进', 'pool_claim', '',
   'create_followup', '您已从公海认领新客户，请尽快完成首次联系并了解客户需求',
   'call', 0, 'active', '公海认领首联提醒'),
  (9010000000000000905, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '客户下单→发送感谢消息', 'order_created', '',
   'send_message', '感谢您的订购！我们的客服将尽快与您联系安排实施部署。',
   '', 0, 'active', '下单自动感谢'),
  (9010000000000000906, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '回款到账→发送确认消息', 'payment_received', '',
   'send_message', '您的付款已确认到账，发票将在3个工作日内开具，请注意查收。',
   '', 0, 'active', '回款自动确认')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ SOP任务（crm_sop_task）
INSERT INTO crm_sop_task
  (id, created_at, updated_at, tenant_id, rule_id, customer_id, opportunity_id,
   action_type, action_content, status, execute_at, assign_user_id)
VALUES
  (9010000000000000911, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000901, 9010000000000000001, 9010000000000000021,
   'create_followup', '商机已进入方案报价阶段，请尽快准备报价单并发送给客户',
   'done', CURRENT_TIMESTAMP - INTERVAL '1 day', 2087390044095516672),
  (9010000000000000912, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000903, 9010000000000000005, NULL,
   'create_followup', '新客户已录入系统，请在24小时内完成首次联系',
   'pending', CURRENT_TIMESTAMP + INTERVAL '12 hour', 2087390044095516672),
  (9010000000000000913, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000905, 9010000000000000001, NULL,
   'send_message', '感谢您的订购！我们的客服将尽快与您联系安排实施部署。',
   'done', CURRENT_TIMESTAMP - INTERVAL '20 day', 2087390044095516672)
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 审批流（crm_approval_flow）
INSERT INTO crm_approval_flow
  (id, created_at, updated_at, tenant_id, biz_type, name, nodes, is_enabled)
VALUES
  (9010000000000000921, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'contract', '合同审批流(>5万)', '[{"name":"销售经理审批","approver_type":"user","approver_ids":["9010000000000000310"]},{"name":"总经理审批","approver_type":"user","approver_ids":["2087390044095516672"]}]',
   true),
  (9010000000000000922, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'quote', '报价审批流(>3万)', '[{"name":"销售经理审批","approver_type":"user","approver_ids":["9010000000000000310"]}]',
   true),
  (9010000000000000923, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'receivables', '回款确认审批', '[{"name":"财务确认","approver_type":"user","approver_ids":["9010000000000000311"]}]',
   true),
  (9010000000000000924, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'expense', '费用报销审批', '[{"name":"部门经理审批","approver_type":"user","approver_ids":["9010000000000000310"]},{"name":"财务审批","approver_type":"user","approver_ids":["9010000000000000311"]}]',
   true)
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 自动分配规则（crm_auto_allocation）
INSERT INTO crm_auto_allocation
  (id, created_at, updated_at, tenant_id, entity_type, name, conditions,
   strategy, target_ids, is_enabled, priority)
VALUES
  (9010000000000000931, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'lead', '小红书线索轮询分配', '[{"field":"source","op":"eq","value":"xiaohongshu"}]',
   'round_robin', '["2087390044095516672","2087129133212504064"]', true, 1),
  (9010000000000000932, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'customer', '大客户指定分配', '[{"field":"level","op":"gte","value":"4"}]',
   'round_robin', '["2087390044095516672","9010000000000000310"]', true, 2)
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 通话记录（crm_call_record）
INSERT INTO crm_call_record
  (id, created_at, updated_at, tenant_id, user_id, customer_id, direction,
   phone, duration, start_at, recording_url, status, remark)
VALUES
  (9010000000000000941, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   2087390044095516672, 9010000000000000001, 'outbound', '13900000001', 320,
   CURRENT_TIMESTAMP - INTERVAL '1 day', 'https://example.com/rec/call-001.mp3',
   'connected', '与王总电话沟通SCRM采购需求'),
  (9010000000000000942, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   2087390044095516672, 9010000000000000002, 'outbound', '13900000002', 180,
   CURRENT_TIMESTAMP - INTERVAL '2 day', 'https://example.com/rec/call-002.mp3',
   'connected', '与李总确认实施方案评估进度'),
  (9010000000000000943, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   2087129133212504064, 9010000000000000003, 'outbound', '13900000003', 0,
   CURRENT_TIMESTAMP - INTERVAL '3 day', '', 'no_answer', '赵总未接听，稍后再拨'),
  (9010000000000000944, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   2087129333339525120, NULL, 'inbound', '13700000005', 95,
   CURRENT_TIMESTAMP - INTERVAL '4 day', 'https://example.com/rec/call-004.mp3',
   'connected', '客户来电咨询产品价格')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 签到（crm_signin）
INSERT INTO crm_signin
  (id, created_at, updated_at, tenant_id, user_id, customer_id,
   latitude, longitude, address, photos, remark)
VALUES
  (9010000000000000951, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   2087390044095516672, 9010000000000000001, 39.9042, 116.4074,
   '北京市朝阳区蓝天科技公司', '["https://example.com/photo/signin-001.jpg"]',
   '拜访蓝天科技王总，签到'),
  (9010000000000000952, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   2087390044095516672, 9010000000000000002, 31.2304, 121.4737,
   '上海市浦东新区红日集团', '["https://example.com/photo/signin-002.jpg"]',
   '上门演示客户管理系统')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 待办（crm_todo）
INSERT INTO crm_todo
  (id, created_at, updated_at, tenant_id, user_id, type, title, content,
   ref_type, ref_id, due_at, status)
VALUES
  (9010000000000000961, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   2087390044095516672, 'followup', '跟进蓝天科技王总',
   '约定下周演示SCRM标准版，需准备演示环境',
   'customer', 9010000000000000001, CURRENT_TIMESTAMP + INTERVAL '2 day', 'pending'),
  (9010000000000000962, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   2087390044095516672, 'receivables_expire', '红日集团二期款即将到期',
   '订单MO-20260805-002二期款10000元将于30天后到期',
   'order', 9010000000000000032, CURRENT_TIMESTAMP + INTERVAL '30 day', 'pending'),
  (9010000000000000963, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   2087129133212504064, 'followup', '跟进绿地贸易赵总',
   '智能客服需求报价已发送，等待客户反馈',
   'customer', 9010000000000000003, CURRENT_TIMESTAMP + INTERVAL '1 day', 'pending'),
  (9010000000000000964, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   2087390044095516672, 'task', '准备SCRM演示PPT',
   '蓝天科技演示需要，重点突出SCRM与企微的集成能力',
   'customer', 9010000000000000001, CURRENT_TIMESTAMP + INTERVAL '3 day', 'pending')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 发票（crm_invoice）
INSERT INTO crm_invoice
  (id, created_at, updated_at, tenant_id, invoice_no, order_id, customer_id,
   type, amount, tax_rate, title, tax_no, issue_date, status)
VALUES
  (9010000000000000971, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'INV-20260801-001', 9010000000000000031, 9010000000000000001,
   'special', 50000, 0.06, '蓝天科技有限公司', '91110000MA01XXXXXX',
   CURRENT_TIMESTAMP - INTERVAL '15 day', 'issued'),
  (9010000000000000972, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'INV-20260819-002', 9010000000000000032, 9010000000000000002,
   'normal', 10000, 0.03, '红日集团股份有限公司', '91310000MA02XXXXXX',
   NULL, 'pending')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 财务记录（crm_finance_record）
INSERT INTO crm_finance_record
  (id, created_at, updated_at, tenant_id, type, category, amount,
   occur_date, order_id, customer_id, status, remark)
VALUES
  (9010000000000000981, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'income', '软件销售收入', 50000, CURRENT_TIMESTAMP - INTERVAL '20 day',
   9010000000000000031, 9010000000000000001, 'confirmed', '蓝天科技SCRM合同全款'),
  (9010000000000000982, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'income', '实施服务收入', 10000, CURRENT_TIMESTAMP - INTERVAL '3 day',
   9010000000000000032, 9010000000000000002, 'confirmed', '红日集团实施首期款'),
  (9010000000000000983, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'expense', '市场推广费', 5000, CURRENT_TIMESTAMP - INTERVAL '5 day',
   NULL, NULL, 'confirmed', '618营销活动推广费用'),
  (9010000000000000984, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'expense', '差旅费', 1200, CURRENT_TIMESTAMP - INTERVAL '2 day',
   NULL, NULL, 'confirmed', '上海客户拜访差旅')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 供应商（crm_supplier）
INSERT INTO crm_supplier
  (id, created_at, updated_at, tenant_id, name, contact, phone, address, remark)
VALUES
  (9010000000000000991, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '阿里云', '张经理', '400-100-1000', '杭州市余杭区', '云服务器供应商'),
  (9010000000000000992, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '腾讯云', '李经理', '400-900-1000', '深圳市南山区', '云服务器及企微API')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 采购订单（crm_purchase_order）
INSERT INTO crm_purchase_order
  (id, created_at, updated_at, tenant_id, no, supplier_id, warehouse_id,
   amount, paid_amount, status, order_date, remark)
VALUES
  (9010000000000001001, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'PO-20260801-001', 9010000000000000991, 9010000000000000711,
   20000, 20000, 'paid', CURRENT_TIMESTAMP - INTERVAL '20 day',
   '阿里云服务器年费采购'),
  (9010000000000001002, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   'PO-20260805-002', 9010000000000000992, 9010000000000000711,
   15000, 15000, 'instocked', CURRENT_TIMESTAMP - INTERVAL '18 day',
   '腾讯云服务器扩容采购')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 项目（crm_project）
INSERT INTO crm_project
  (id, created_at, updated_at, tenant_id, name, type, customer_id, order_id,
   manager_id, amount, start_date, end_date, status, progress, project_no, stage)
VALUES
  (9010000000000001011, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '蓝天科技SCRM实施项目', 'implementation', 9010000000000000001,
   9010000000000000031, 2087390044095516672, 50000,
   CURRENT_TIMESTAMP - INTERVAL '20 day', CURRENT_TIMESTAMP + INTERVAL '10 day',
   'active', 80, 'PRJ-001', '部署调试'),
  (9010000000000001012, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '红日集团CRM迁移项目', 'migration', 9010000000000000002,
   9010000000000000032, 2087129133212504064, 30000,
   CURRENT_TIMESTAMP - INTERVAL '5 day', CURRENT_TIMESTAMP + INTERVAL '25 day',
   'active', 30, 'PRJ-002', '数据迁移')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 敏感词（crm_sensitive_word）
INSERT INTO crm_sensitive_word
  (id, created_at, updated_at, tenant_id, word, category, action, status)
VALUES
  (9010000000000001021, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '飞单', 'fly_single', 'block', 'active'),
  (9010000000000001022, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '私单', 'fly_single', 'block', 'active'),
  (9010000000000001023, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '傻逼', 'abuse', 'block', 'active'),
  (9010000000000001024, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '身份证号', 'privacy', 'warn', 'active'),
  (9010000000000001025, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '银行卡号', 'privacy', 'warn', 'active')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ AI分析（crm_ai_analysis）
INSERT INTO crm_ai_analysis
  (id, created_at, updated_at, tenant_id, customer_id, type, summary,
   details, score, status, analysis_at, model_version)
VALUES
  (9010000000000001031, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000001, 'customer_profile',
   '高价值客户：企业规模大，预算充足，已签约年度续费，有扩容潜力。互动频次高，响应积极。',
   '["{\"dim\":\"客户等级\",\"value\":\"3\",\"weight\":15}","{\"dim\":\"互动频次\",\"value\":\"8次\",\"weight\":8}","{\"dim\":\"商机金额\",\"value\":\"12万\",\"weight\":12}","{\"dim\":\"订单金额\",\"value\":\"5万\",\"weight\":5}"]',
   40, 'completed', CURRENT_TIMESTAMP - INTERVAL '1 day', 'rule-based-v1'),
  (9010000000000001032, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000002, 'customer_profile',
   '潜力客户：需求明确，正在评估实施方案，有较高转化概率。建议加快跟进节奏。',
   '["{\"dim\":\"客户等级\",\"value\":\"4\",\"weight\":12}","{\"dim\":\"互动频次\",\"value\":\"5次\",\"weight\":5}","{\"dim\":\"商机金额\",\"value\":\"8万\",\"weight\":8}","{\"dim\":\"订单金额\",\"value\":\"0\",\"weight\":0}"]',
   25, 'completed', CURRENT_TIMESTAMP - INTERVAL '2 day', 'rule-based-v1'),
  (9010000000000001033, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000005, 'pain_point',
   '小红书来源客户关注产品测评和口碑，对优惠券敏感。痛点：需要更多产品使用场景证明。',
   '["{\"dim\":\"来源渠道\",\"value\":\"小红书\"}","{\"dim\":\"关注点\",\"value\":\"测评/口碑/优惠\"}"]',
   18, 'completed', CURRENT_TIMESTAMP - INTERVAL '3 day', 'rule-based-v1')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 裂变活动（crm_fission）
INSERT INTO crm_fission
  (id, created_at, updated_at, tenant_id, activity_id, name, reward_type,
   reward_threshold, max_reward_per_user, status, remark)
VALUES
  (9010000000000001041, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000201, '老带新·邀请得券', 'coupon', 1, 3,
   'active', '每邀请1位新客户注册，邀请人获得满100减20优惠券1张，最多3张')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 落地页（crm_landing_page）
INSERT INTO crm_landing_page
  (id, created_at, updated_at, tenant_id, activity_id, title, slug,
   template, form_fields, submit_count, status)
VALUES
  (9010000000000001051, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000204, '小红书种草·新品测评留资', 'xhs-newproduct-2026',
   'form', '["{\"name\":\"姓名\",\"required\":true}","{\"name\":\"手机\",\"required\":true}","{\"name\":\"公司\",\"required\":false}"]',
   41, 'active'),
  (9010000000000001052, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   9010000000000000201, '618老客回馈·领券页', '618-coupon-2026',
   'simple', '["{\"name\":\"姓名\",\"required\":true}","{\"name\":\"手机\",\"required\":true}"]',
   210, 'active')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 智能名片（crm_smart_card）
INSERT INTO crm_smart_card
  (id, created_at, updated_at, tenant_id, user_id, name, title, company,
   phone, email, avatar, intro, tags, products, cases,
   view_count, share_count, share_code, status)
VALUES
  (9010000000000001061, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   2087390044095516672, '张经理', '销售经理', '智云科技',
   '13800000001', 'manager@demo.com', 'https://example.com/avatar/001.png',
   '10年企业服务经验，专注SCRM和客户运营领域',
   '["SCRM","企业微信","客户运营"]',
   '["{\"name\":\"SCRM标准版\",\"desc\":\"企业微信客户管理一站式方案\"}","{\"name\":\"SCRM企业版\",\"desc\":\"不限坐席+高级分析+API开放\"}"]',
   '["{\"name\":\"蓝天科技SCRM部署\",\"desc\":\"3天完成100人团队上线\"}","{\"name\":\"红日集团CRM迁移\",\"desc\":\"5万条数据无缝迁移\"}"]',
   326, 58, 'SCRM001', 'active'),
  (9010000000000001062, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   2087129133212504064, '销售小王', '高级销售', '智云科技',
   '13800000003', 'sales1@demo.com', 'https://example.com/avatar/002.png',
   '5年SaaS销售经验，擅长客户需求分析和方案定制',
   '["智能客服","SaaS","售前咨询"]',
   '["{\"name\":\"智能客服\",\"desc\":\"AI驱动7x24小时客户服务\"}"]',
   '["{\"name\":\"绿地贸易客服系统\",\"desc\":\"月均处理3000+咨询\"}"]',
   156, 23, 'SCRM002', 'active')
ON CONFLICT (id) DO NOTHING;

-- ============================================================ 客户评分更新（补充 score 字段）
UPDATE crm_customer SET score = 40, score_updated_at = CURRENT_TIMESTAMP WHERE id = 9010000000000000001 AND score = 0;
UPDATE crm_customer SET score = 25, score_updated_at = CURRENT_TIMESTAMP WHERE id = 9010000000000000002 AND score = 0;
UPDATE crm_customer SET score = 15, score_updated_at = CURRENT_TIMESTAMP WHERE id = 9010000000000000003 AND score = 0;
UPDATE crm_customer SET score = 45, score_updated_at = CURRENT_TIMESTAMP WHERE id = 9010000000000000004 AND score = 0;
UPDATE crm_customer SET score = 18, score_updated_at = CURRENT_TIMESTAMP WHERE id = 9010000000000000005 AND score = 0;
UPDATE crm_customer SET score = 8,  score_updated_at = CURRENT_TIMESTAMP WHERE id = 9010000000000000006 AND score = 0;

-- ============================================================ 公海池客户（补充1条公海客户）
INSERT INTO crm_customer
  (id, created_at, updated_at, tenant_id, name, mobile, email, company, industry,
   gender, level, address, source, owner_user_id, pool_status, pool_at, remark)
SELECT 9010000000000000007, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   '马总', '13900000007', 'ma@jinqiao.com', '金桥传媒', '广告',
   1, 2, '成都市武侯区', 'import', NULL, 1, CURRENT_TIMESTAMP - INTERVAL '3 day',
   '批量导入客户，无人认领，在公海池中'
WHERE NOT EXISTS (SELECT 1 FROM crm_customer WHERE id = 9010000000000000007);

-- ============================================================ 线索池补充（2条池中线索）
UPDATE crm_lead SET pool_status = 1, pool_at = CURRENT_TIMESTAMP - INTERVAL '2 day'
 WHERE id = 9010000000000000013 AND pool_status = 0;

INSERT INTO crm_lead
  (id, created_at, updated_at, tenant_id, customer_id, owner_user_id, title, company,
   name, mobile, source, status, pool_status, pool_at, remark)
SELECT 9010000000000000015, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2087127714048774144,
   NULL, NULL, '抖音广告咨询', '光影工作室', '冯经理', '13700000005',
   'douyin', 'new', 1, CURRENT_TIMESTAMP - INTERVAL '1 day',
   '抖音渠道来的线索，进入线索池待领取'
WHERE NOT EXISTS (SELECT 1 FROM crm_lead WHERE id = 9010000000000000015);

COMMIT;
