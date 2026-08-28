-- migrate:statement
-- migrate:statement
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS auth_version bigint NOT NULL DEFAULT 1;
-- migrate:statement
ALTER TABLE platform_admin ADD COLUMN IF NOT EXISTS auth_version bigint NOT NULL DEFAULT 1;
-- migrate:statement
ALTER TABLE staff ADD COLUMN IF NOT EXISTS auth_version bigint NOT NULL DEFAULT 1;
-- migrate:statement
CREATE OR REPLACE FUNCTION invalidate_account_sessions() RETURNS trigger LANGUAGE plpgsql AS $session$
BEGIN
    IF NEW.password IS DISTINCT FROM OLD.password OR NEW.status IS DISTINCT FROM OLD.status
       OR NEW.deleted_at IS DISTINCT FROM OLD.deleted_at THEN
        NEW.auth_version := OLD.auth_version + 1;
    ELSIF NEW.auth_version < OLD.auth_version THEN
        RAISE EXCEPTION 'session version cannot decrease';
    END IF;
    IF TG_TABLE_NAME = 'user' THEN
        IF NEW.role_id IS DISTINCT FROM OLD.role_id AND NEW.auth_version=OLD.auth_version THEN NEW.auth_version := OLD.auth_version + 1; END IF;
    END IF;
    RETURN NEW;
END
$session$;
-- migrate:statement
CREATE TRIGGER invalidate_user_sessions BEFORE UPDATE ON "user"
FOR EACH ROW EXECUTE FUNCTION invalidate_account_sessions();
-- migrate:statement
CREATE TRIGGER invalidate_platform_sessions BEFORE UPDATE ON platform_admin
FOR EACH ROW EXECUTE FUNCTION invalidate_account_sessions();
-- migrate:statement
CREATE OR REPLACE FUNCTION invalidate_staff_sessions() RETURNS trigger LANGUAGE plpgsql AS $session$
BEGIN
    IF NEW.status IS DISTINCT FROM OLD.status OR NEW.role_id IS DISTINCT FROM OLD.role_id
       OR NEW.deleted_at IS DISTINCT FROM OLD.deleted_at THEN
        NEW.auth_version := OLD.auth_version + 1;
    ELSIF NEW.auth_version < OLD.auth_version THEN
        RAISE EXCEPTION 'session version cannot decrease';
    END IF;
    RETURN NEW;
END
$session$;
-- migrate:statement
CREATE TRIGGER invalidate_staff_sessions BEFORE UPDATE ON staff
FOR EACH ROW EXECUTE FUNCTION invalidate_staff_sessions();
