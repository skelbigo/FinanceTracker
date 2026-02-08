ALTER TABLE notification_delivery
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE INDEX IF NOT EXISTS idx_notification_delivery_status_updated
    ON notification_delivery(status, updated_at DESC);
