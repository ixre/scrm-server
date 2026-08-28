-- migrate:statement
ALTER TABLE chat_msg ADD COLUMN IF NOT EXISTS tenant_id bigint;
-- migrate:statement
ALTER TABLE chat_msg_content ADD COLUMN IF NOT EXISTS tenant_id bigint;
-- migrate:statement
UPDATE chat_msg m SET tenant_id=cl.tenant_id FROM channel_link cl
WHERE m.tenant_id IS NULL AND cl.channel='wecom' AND cl.entity_type='tenant'
AND cl.deleted_at IS NULL AND cl.ext_id=rtrim(m.ext_corp_id::text);
-- migrate:statement
UPDATE chat_msg_content c SET tenant_id=m.tenant_id FROM chat_msg m
WHERE c.chat_msg_id=m.id AND c.ext_corp_id=m.ext_corp_id AND c.tenant_id IS NULL;
-- migrate:statement
ALTER TABLE chat_msg ALTER COLUMN tenant_id SET NOT NULL;
-- migrate:statement
ALTER TABLE chat_msg_content ALTER COLUMN tenant_id SET NOT NULL;
-- migrate:statement
DO $migration$
DECLARE idx record;
BEGIN
 FOR idx IN SELECT ns.nspname AS schema_name,t.relname AS table_name,i.relname AS index_name,con.conname AS constraint_name
 FROM pg_index x JOIN pg_class t ON t.oid=x.indrelid JOIN pg_namespace ns ON ns.oid=t.relnamespace
 JOIN pg_class i ON i.oid=x.indexrelid LEFT JOIN pg_constraint con ON con.conindid=x.indexrelid
 WHERE ns.nspname=current_schema() AND t.relname='chat_msg' AND x.indisunique AND NOT x.indisprimary
 AND NOT EXISTS (SELECT 1 FROM unnest(x.indkey) AS key(attnum) JOIN pg_attribute a ON a.attrelid=t.oid AND a.attnum=key.attnum WHERE a.attname IN ('ext_corp_id','id'))
 LOOP
 IF idx.constraint_name IS NOT NULL THEN EXECUTE format('ALTER TABLE %I.%I DROP CONSTRAINT %I',idx.schema_name,idx.table_name,idx.constraint_name);
 ELSE EXECUTE format('DROP INDEX %I.%I',idx.schema_name,idx.index_name); END IF;
 END LOOP;
END
$migration$;
-- migrate:statement
CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_msg_corp_message ON chat_msg(ext_corp_id,msg_id);
-- migrate:statement
CREATE INDEX IF NOT EXISTS idx_chat_msg_corp_session_time ON chat_msg(ext_corp_id,session_id,msg_time DESC,id DESC);
-- migrate:statement
CREATE TABLE IF NOT EXISTS wecom_archive_cursor(corp_id varchar(64) PRIMARY KEY,tenant_id bigint NOT NULL,seq bigint NOT NULL DEFAULT 0);
-- migrate:statement
INSERT INTO wecom_archive_cursor(corp_id,tenant_id,seq)
SELECT rtrim(ext_corp_id::text),tenant_id,max(seq) FROM chat_msg GROUP BY ext_corp_id,tenant_id
ON CONFLICT(corp_id) DO NOTHING;
-- migrate:statement
CREATE TRIGGER enforce_archive_message_tenant BEFORE INSERT OR UPDATE ON chat_msg
FOR EACH ROW EXECUTE FUNCTION enforce_legacy_tenant();
-- migrate:statement
CREATE TRIGGER enforce_archive_content_tenant BEFORE INSERT OR UPDATE ON chat_msg_content
FOR EACH ROW EXECUTE FUNCTION enforce_legacy_tenant();
