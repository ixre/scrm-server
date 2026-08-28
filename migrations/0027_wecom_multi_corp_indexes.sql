-- migrate:statement
-- Enterprise-local identifiers must coexist across enterprises. Retain primary
-- keys and all unrelated indexes. Remove only the obsolete uniqueness rules.
DO $migration$
DECLARE idx record;
BEGIN
    FOR idx IN
        SELECT ns.nspname AS schema_name, t.relname AS table_name,
               i.relname AS index_name, con.conname AS constraint_name
        FROM pg_index x
        JOIN pg_class t ON t.oid=x.indrelid
        JOIN pg_namespace ns ON ns.oid=t.relnamespace
        JOIN pg_class i ON i.oid=x.indexrelid
        LEFT JOIN pg_constraint con ON con.conindid=x.indexrelid
        WHERE ns.nspname=current_schema() AND x.indisunique AND NOT x.indisprimary
          AND t.relname IN ('customer','customer_staff','customer_info','tag','tag_group','group_chat','group_chat_member','customer_statistic')
          AND NOT EXISTS (
              SELECT 1 FROM unnest(x.indkey) AS key(attnum)
              JOIN pg_attribute a ON a.attrelid=t.oid AND a.attnum=key.attnum
              WHERE a.attname IN ('ext_corp_id','id')
          )
    LOOP
        IF idx.constraint_name IS NOT NULL THEN
            EXECUTE format('ALTER TABLE %I.%I DROP CONSTRAINT %I', idx.schema_name,idx.table_name,idx.constraint_name);
        ELSE
            EXECUTE format('DROP INDEX %I.%I', idx.schema_name,idx.index_name);
        END IF;
    END LOOP;
END
$migration$;
-- migrate:statement
CREATE UNIQUE INDEX IF NOT EXISTS idx_customer_corp_external ON customer(ext_corp_id,ext_id);
-- migrate:statement
CREATE UNIQUE INDEX IF NOT EXISTS idx_customer_staff_corp_external ON customer_staff(ext_corp_id,ext_customer_id,ext_staff_id);
-- migrate:statement
CREATE UNIQUE INDEX IF NOT EXISTS idx_customer_info_corp_external ON customer_info(ext_corp_id,ext_customer_id,ext_staff_id);
-- migrate:statement
CREATE UNIQUE INDEX IF NOT EXISTS idx_tag_corp_external ON tag(ext_corp_id,ext_id);
-- migrate:statement
CREATE UNIQUE INDEX IF NOT EXISTS idx_tag_group_corp_external ON tag_group(ext_corp_id,ext_id);
-- migrate:statement
CREATE UNIQUE INDEX IF NOT EXISTS idx_group_chat_corp_external ON group_chat(ext_corp_id,ext_chat_id);
-- migrate:statement
CREATE UNIQUE INDEX IF NOT EXISTS idx_customer_statistic_corp_day ON customer_statistic(ext_corp_id,ext_staff_id,date);

-- migrate:statement
CREATE UNIQUE INDEX IF NOT EXISTS idx_group_chat_member_corp_external ON group_chat_member(ext_corp_id,ext_chat_id,userid);

-- migrate:statement
ALTER TABLE crm_channel_config ADD COLUMN IF NOT EXISTS msg_arch_enabled integer NOT NULL DEFAULT 2;
-- migrate:statement
ALTER TABLE crm_channel_config ADD COLUMN IF NOT EXISTS msg_arch_secret text;
-- migrate:statement
ALTER TABLE crm_channel_config ADD COLUMN IF NOT EXISTS msg_arch_private_keys text;
-- migrate:statement
CREATE UNIQUE INDEX IF NOT EXISTS idx_staff_department_corp_staff_dept ON staff_department(ext_corp_id,ext_staff_id,ext_department_id);
