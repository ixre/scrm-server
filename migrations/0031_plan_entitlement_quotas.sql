-- migrate:statement
-- Quotas apply to every writer, including imports, conversion, callbacks and
-- soft-delete restoration. A tenant row lock serializes concurrent growth.
-- Existing over-limit data is retained; only further growth is rejected.
CREATE OR REPLACE FUNCTION enforce_plan_quota() RETURNS trigger LANGUAGE plpgsql AS $quota$
DECLARE
    t tenant%ROWTYPE;
    p plan%ROWTYPE;
    cap integer;
    override_cap integer;
    used bigint;
BEGIN
    IF NEW.deleted_at IS NOT NULL THEN RETURN NEW; END IF;
    IF TG_TABLE_NAME = 'user' THEN
        IF NEW.status <> 1 THEN RETURN NEW; END IF;
    END IF;
    IF TG_OP = 'UPDATE' THEN
        IF NEW.tenant_id IS DISTINCT FROM OLD.tenant_id THEN
            RAISE EXCEPTION '不能修改数据所属租户';
        END IF;
        IF OLD.deleted_at IS NULL THEN
            IF TG_TABLE_NAME <> 'user' THEN RETURN NEW; END IF;
            IF OLD.status = 1 THEN RETURN NEW; END IF;
        END IF;
    END IF;
    SELECT * INTO t FROM tenant WHERE id=NEW.tenant_id AND deleted_at IS NULL FOR UPDATE;
    IF NOT FOUND OR t.status <> 1 THEN RAISE EXCEPTION '企业不存在或已停用'; END IF;
    IF t.expire_at IS NOT NULL AND t.expire_at <= clock_timestamp() THEN
        RAISE EXCEPTION '企业套餐已到期，请先续费';
    END IF;
    SELECT * INTO p FROM plan WHERE code=t.plan AND deleted_at IS NULL;
    IF NOT FOUND THEN RAISE EXCEPTION '企业套餐不存在，请联系平台管理员'; END IF;
    IF TG_TABLE_NAME = 'user' THEN cap := p.max_users; override_cap := t.max_users;
    ELSE cap := p.max_customers; override_cap := t.max_customers; END IF;
    IF cap < 0 OR override_cap < 0 THEN RAISE EXCEPTION '套餐配额配置无效'; END IF;
    IF override_cap > 0 AND (cap = 0 OR override_cap < cap) THEN cap := override_cap; END IF;
    IF cap = 0 THEN RETURN NEW; END IF;
    -- An INSERT ... ON CONFLICT targeting an already occupied slot is not growth.
    IF TG_OP = 'INSERT' THEN
        IF TG_TABLE_NAME = 'user' THEN
            IF EXISTS (SELECT 1 FROM "user" WHERE id=NEW.id AND tenant_id=NEW.tenant_id AND deleted_at IS NULL AND status=1) THEN RETURN NEW; END IF;
        ELSE
            IF EXISTS (SELECT 1 FROM crm_customer WHERE id=NEW.id AND tenant_id=NEW.tenant_id AND deleted_at IS NULL) THEN RETURN NEW; END IF;
        END IF;
    END IF;
    IF TG_TABLE_NAME = 'user' THEN
        SELECT count(*) INTO used FROM "user" WHERE tenant_id=NEW.tenant_id AND deleted_at IS NULL AND status=1;
    ELSE
        SELECT count(*) INTO used FROM crm_customer WHERE tenant_id=NEW.tenant_id AND deleted_at IS NULL;
    END IF;
    IF used >= cap THEN
        IF TG_TABLE_NAME = 'user' THEN RAISE EXCEPTION '当前套餐用户数已达上限(%)，请升级套餐或停用多余账号', cap;
        ELSE RAISE EXCEPTION '当前套餐客户数已达上限(%)，请升级套餐或删除多余客户', cap; END IF;
    END IF;
    RETURN NEW;
END
$quota$;
-- migrate:statement
CREATE TRIGGER enforce_user_plan_quota BEFORE INSERT OR UPDATE ON "user"
FOR EACH ROW EXECUTE FUNCTION enforce_plan_quota();
-- migrate:statement
CREATE TRIGGER enforce_customer_plan_quota BEFORE INSERT OR UPDATE ON crm_customer
FOR EACH ROW EXECUTE FUNCTION enforce_plan_quota();
