-- Preserve the quote-to-contract link for installations with AutoMigration disabled.
ALTER TABLE crm_quote ADD COLUMN IF NOT EXISTS order_id bigint;
CREATE INDEX IF NOT EXISTS idx_crm_quote_order_id ON crm_quote(order_id);
