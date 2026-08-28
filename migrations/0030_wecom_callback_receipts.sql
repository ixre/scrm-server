CREATE TABLE IF NOT EXISTS wecom_event_receipt(receipt_key varchar(64) PRIMARY KEY,tenant_id bigint NOT NULL,created_at timestamp with time zone NOT NULL);
CREATE INDEX IF NOT EXISTS idx_wecom_receipt_created ON wecom_event_receipt(created_at);
