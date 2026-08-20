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
-- 平台超管（独立于租户，登录入口 /platform/login）：
--   master   / 123456  — 平台超管（管理所有租户：创建/停用/删除/编辑）
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
-- 登录入口：/platform/login，账号 master / 123456。
-- bcrypt hash of "123456"（与 CRM 用户一致）:
--   $2a$10$XB0Gf8Gkq0kVFVlhdZZUBeCcDfEwUWRWUtFqSnb2Tb71mpBZOAD2.
BEGIN;
INSERT INTO platform_admin
  (id, created_at, updated_at, username, password, name, mobile, email, role, status)
SELECT 9010000000000000400, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP,
   'master', '$2a$10$XB0Gf8Gkq0kVFVlhdZZUBeCcDfEwUWRWUtFqSnb2Tb71mpBZOAD2.',
   '平台超管', '', '', 'super_admin', 1
 WHERE NOT EXISTS (
   SELECT 1 FROM platform_admin WHERE username = 'master' AND deleted_at IS NULL
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
