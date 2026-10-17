-- Run from psql with -X -v ON_ERROR_STOP=1 -f scripts/migrate_model_refactor.sql.
-- Applies only 0011-0013. Use cmd/migrate -schema-only to apply all pending versions.
\set ON_ERROR_STOP on
BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';
SELECT pg_advisory_xact_lock(7362726);
CREATE TABLE IF NOT EXISTS schema_migrations (version VARCHAR(255) PRIMARY KEY, applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP);
\ir ../migrations/0011_explicit_model_indexes.sql
\ir ../migrations/0012_child_enterprise_backfill.sql
\ir ../migrations/0013_model_field_cleanup.sql
INSERT INTO schema_migrations (version) VALUES
  ('0011_explicit_model_indexes.sql'),
  ('0012_child_enterprise_backfill.sql'),
  ('0013_model_field_cleanup.sql')
ON CONFLICT (version) DO NOTHING;
COMMIT;
