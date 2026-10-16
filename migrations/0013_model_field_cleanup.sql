-- corp_id was unused and its duplicate JSON tag hid the actual ext_corp_id.
-- Refuse to discard historical values. The division fails if any value exists.
-- Reading the column via JSON also makes this check safe after the column is gone.
SELECT 1 / (1 - LEAST(COUNT(*), 1)) AS legacy_corp_id_must_be_empty
FROM quick_reply
WHERE COALESCE(trim(to_jsonb(quick_reply)->>'corp_id'), '') <> '';
ALTER TABLE quick_reply DROP COLUMN IF EXISTS corp_id;

-- Titles are variable-length text. Remove char padding without changing the title.
ALTER TABLE welcome_msg ALTER COLUMN name TYPE varchar(128) USING rtrim(name::text);
