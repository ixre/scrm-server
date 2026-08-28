-- Preserve legacy external URLs. New uploads use private storage and a content digest.
ALTER TABLE crm_order_attachment ADD COLUMN IF NOT EXISTS storage_key varchar(255);
ALTER TABLE crm_order_attachment ADD COLUMN IF NOT EXISTS sha256 varchar(64);
CREATE UNIQUE INDEX IF NOT EXISTS idx_crm_attachment_storage_key ON crm_order_attachment(storage_key) WHERE storage_key <> '';
