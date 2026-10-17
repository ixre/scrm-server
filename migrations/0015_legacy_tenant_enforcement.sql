-- migrate:statement
-- Maintenance-window migration. Explicit statement boundaries preserve function bodies.
-- No default tenant, inferred ownership, physical foreign keys, or rewritten business IDs.
LOCK TABLE channel_link, tenant, crm_channel_config, customer, staff, department, group_chat, mass_msg, material, contact_way IN SHARE ROW EXCLUSIVE MODE;
-- migrate:statement
CREATE UNIQUE INDEX IF NOT EXISTS idx_wecom_tenant_corp ON channel_link(ext_id)
WHERE channel = 'wecom' AND entity_type = 'tenant' AND deleted_at IS NULL;
-- migrate:statement
DO $migration$
DECLARE
    target text;
    invalid_count bigint;
BEGIN
    IF EXISTS (
        SELECT 1 FROM channel_link cl LEFT JOIN tenant t ON t.id = cl.entity_id
        WHERE cl.channel = 'wecom' AND cl.entity_type = 'tenant' AND cl.deleted_at IS NULL
          AND (cl.ext_id IS NULL OR btrim(cl.ext_id) = '' OR cl.ext_id <> btrim(cl.ext_id)
               OR COALESCE(cl.ext_corp_id,'') NOT IN ('',cl.ext_id)
               OR cl.tenant_id IS DISTINCT FROM cl.entity_id OR t.id IS NULL)
    ) THEN
        RAISE EXCEPTION 'Invalid WeCom tenant mapping: verify entity_id, tenant_id, enterprise ID and tenant existence';
    END IF;
    IF EXISTS (
        SELECT 1 FROM crm_channel_config cfg
        WHERE cfg.channel='wecom' AND cfg.deleted_at IS NULL AND NOT EXISTS (
            SELECT 1 FROM channel_link cl WHERE cl.channel='wecom' AND cl.entity_type='tenant'
              AND cl.deleted_at IS NULL AND cl.ext_id=cfg.corp_id AND cl.entity_id=cfg.tenant_id
        )
    ) THEN
        RAISE EXCEPTION 'Existing WeCom channel configuration has a missing or conflicting tenant mapping';
    END IF;
    FOREACH target IN ARRAY ARRAY['customer','staff','department','group_chat','mass_msg','material','contact_way'] LOOP
        EXECUTE format(
            'SELECT count(*) FROM %I r LEFT JOIN channel_link cl ON cl.channel=''wecom'' AND cl.entity_type=''tenant'' AND cl.deleted_at IS NULL AND cl.ext_id=rtrim(r.ext_corp_id::text)
             WHERE cl.id IS NULL OR (r.tenant_id IS NOT NULL AND r.tenant_id IS DISTINCT FROM cl.entity_id)', target)
            INTO invalid_count;
        IF invalid_count > 0 THEN
            RAISE EXCEPTION 'Tenant migration blocked: table %, % rows have missing or conflicting enterprise ownership', target, invalid_count;
        END IF;
        EXECUTE format(
            'UPDATE %I r SET tenant_id=cl.entity_id FROM channel_link cl
             WHERE r.tenant_id IS NULL AND cl.channel=''wecom'' AND cl.entity_type=''tenant''
               AND cl.deleted_at IS NULL AND cl.ext_id=rtrim(r.ext_corp_id::text)', target);
        EXECUTE format('ALTER TABLE %I ALTER COLUMN tenant_id SET NOT NULL', target);
    END LOOP;
END
$migration$;
-- migrate:statement
CREATE OR REPLACE FUNCTION enforce_legacy_tenant() RETURNS trigger LANGUAGE plpgsql AS $function$
DECLARE
    mapped_tenant bigint;
BEGIN
    IF TG_OP = 'UPDATE' THEN
        IF NEW.ext_corp_id IS DISTINCT FROM OLD.ext_corp_id OR NEW.tenant_id IS DISTINCT FROM OLD.tenant_id THEN
            RAISE EXCEPTION 'Enterprise and tenant ownership cannot be changed for %', TG_TABLE_NAME USING ERRCODE='23514';
        END IF;
        RETURN NEW;
    END IF;
    SELECT entity_id INTO mapped_tenant FROM channel_link
    WHERE channel='wecom' AND entity_type='tenant' AND deleted_at IS NULL
      AND ext_id=rtrim(NEW.ext_corp_id::text);
    IF mapped_tenant IS NULL THEN
        RAISE EXCEPTION 'Configure a valid WeCom tenant mapping before writing %', TG_TABLE_NAME USING ERRCODE='23514';
    END IF;
    IF NEW.tenant_id IS NOT NULL AND NEW.tenant_id <> mapped_tenant THEN
        RAISE EXCEPTION 'Tenant does not own the enterprise for %', TG_TABLE_NAME USING ERRCODE='23514';
    END IF;
    NEW.tenant_id := mapped_tenant;
    RETURN NEW;
END
$function$;
-- migrate:statement
CREATE OR REPLACE FUNCTION protect_wecom_tenant_mapping() RETURNS trigger LANGUAGE plpgsql AS $function$
BEGIN
    IF TG_OP <> 'INSERT' THEN
        IF OLD.channel='wecom' AND OLD.entity_type='tenant' AND OLD.deleted_at IS NULL THEN
            IF TG_OP='DELETE' THEN
                RAISE EXCEPTION 'WeCom enterprise ownership cannot be deleted' USING ERRCODE='23514';
            END IF;
            IF NEW.channel IS DISTINCT FROM OLD.channel OR NEW.entity_type IS DISTINCT FROM OLD.entity_type
               OR NEW.ext_id IS DISTINCT FROM OLD.ext_id OR NEW.tenant_id IS DISTINCT FROM OLD.tenant_id
               OR NEW.entity_id IS DISTINCT FROM OLD.entity_id OR NEW.deleted_at IS NOT NULL THEN
                RAISE EXCEPTION 'WeCom enterprise ownership cannot be reassigned or removed' USING ERRCODE='23514';
            END IF;
        END IF;
    END IF;
    IF TG_OP='DELETE' THEN RETURN OLD; END IF;
    IF NEW.channel='wecom' AND NEW.entity_type='tenant' AND NEW.deleted_at IS NULL THEN
        IF NEW.ext_id IS NULL OR btrim(NEW.ext_id)='' OR NEW.ext_id <> btrim(NEW.ext_id)
           OR COALESCE(NEW.ext_corp_id,'') NOT IN ('',NEW.ext_id)
           OR NEW.tenant_id IS DISTINCT FROM NEW.entity_id OR NEW.entity_id IS NULL THEN
            RAISE EXCEPTION 'Invalid WeCom tenant mapping' USING ERRCODE='23514';
        END IF;
        PERFORM id FROM tenant WHERE id=NEW.entity_id FOR KEY SHARE;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'WeCom mapping tenant does not exist' USING ERRCODE='23514';
        END IF;
    END IF;
    RETURN NEW;
END
$function$;
-- migrate:statement
CREATE OR REPLACE FUNCTION protect_mapped_tenant() RETURNS trigger LANGUAGE plpgsql AS $function$
BEGIN
    IF TG_OP='UPDATE' AND NEW.id IS NOT DISTINCT FROM OLD.id THEN RETURN NEW; END IF;
    IF EXISTS (SELECT 1 FROM channel_link WHERE tenant_id=OLD.id AND channel='wecom' AND entity_type='tenant' AND deleted_at IS NULL) THEN
        RAISE EXCEPTION 'Tenant has a WeCom enterprise mapping and cannot be physically deleted' USING ERRCODE='23514';
    END IF;
    IF TG_OP='DELETE' THEN RETURN OLD; END IF;
    RETURN NEW;
END
$function$;
-- migrate:statement
DO $migration$
DECLARE target text;
BEGIN
    FOREACH target IN ARRAY ARRAY['customer','staff','department','group_chat','mass_msg','material','contact_way'] LOOP
        EXECUTE format('CREATE TRIGGER enforce_legacy_tenant BEFORE INSERT OR UPDATE ON %I FOR EACH ROW EXECUTE FUNCTION enforce_legacy_tenant()', target);
    END LOOP;
END
$migration$;
-- migrate:statement
CREATE TRIGGER protect_wecom_tenant_mapping BEFORE INSERT OR UPDATE OR DELETE ON channel_link
FOR EACH ROW EXECUTE FUNCTION protect_wecom_tenant_mapping();
-- migrate:statement
CREATE TRIGGER protect_mapped_tenant BEFORE DELETE OR UPDATE ON tenant
FOR EACH ROW EXECUTE FUNCTION protect_mapped_tenant();
