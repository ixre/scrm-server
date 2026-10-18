-- Legacy clients remain compatible. Historical receipts retain empty request keys.
ALTER TABLE crm_receivables ADD COLUMN IF NOT EXISTS client_request_id varchar(64);
ALTER TABLE crm_receivables ADD COLUMN IF NOT EXISTS request_digest varchar(64);
-- Receipt request indexes.
CREATE UNIQUE INDEX IF NOT EXISTS idx_receipt_order_request ON crm_receivables (tenant_id, order_id, client_request_id) WHERE client_request_id IS NOT NULL AND client_request_id <> '';
CREATE INDEX IF NOT EXISTS idx_plan_order_period_option ON crm_receivables_plan (tenant_id, order_id, period_no, id DESC) WHERE deleted_at IS NULL;
