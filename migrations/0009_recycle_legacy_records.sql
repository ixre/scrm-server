-- Rebuild the recycle index for records deleted before transactional recycle logging.
-- Negative IDs are reserved for this migration so they cannot collide with positive Snowflake IDs.
-- Source rows stay deleted. Unknown historical deleters stay NULL.
LOCK TABLE crm_recycle IN SHARE ROW EXCLUSIVE MODE;
CREATE SEQUENCE IF NOT EXISTS crm_recycle_legacy_id_seq AS bigint
    INCREMENT BY -1 MINVALUE -9223372036854775808 MAXVALUE -1 START WITH -1;
SELECT setval('crm_recycle_legacy_id_seq',
    LEAST(-1, COALESCE((SELECT MIN(id) - 1 FROM crm_recycle WHERE id < 0), -1)), false);
UPDATE crm_recycle SET entity_type = 'order' WHERE entity_type = 'contract';
WITH deleted_entities AS (
    SELECT tenant_id, 'customer' AS kind, id, name AS title, deleted_at FROM crm_customer WHERE deleted_at IS NOT NULL
    UNION ALL SELECT tenant_id, 'lead', id, title, deleted_at FROM crm_lead WHERE deleted_at IS NOT NULL
    UNION ALL SELECT tenant_id, 'opportunity', id, title, deleted_at FROM crm_opportunity WHERE deleted_at IS NOT NULL
    UNION ALL SELECT tenant_id, 'order', id, title, deleted_at FROM crm_order WHERE deleted_at IS NOT NULL
    UNION ALL SELECT tenant_id, 'contact', id, name, deleted_at FROM crm_contact WHERE deleted_at IS NOT NULL
)
INSERT INTO crm_recycle(id, tenant_id, entity_type, entity_id, entity_name, created_at, updated_at)
SELECT nextval('crm_recycle_legacy_id_seq'), e.tenant_id, e.kind, e.id, COALESCE(e.title, ''), e.deleted_at, e.deleted_at
FROM deleted_entities e
WHERE e.tenant_id IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM crm_recycle r
    WHERE r.tenant_id = e.tenant_id AND r.entity_type = e.kind AND r.entity_id = e.id AND r.deleted_at IS NULL
);
CREATE INDEX IF NOT EXISTS idx_crm_recycle_entity ON crm_recycle(tenant_id, entity_type, entity_id) WHERE deleted_at IS NULL;
