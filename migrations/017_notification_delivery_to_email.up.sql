ALTER TABLE notification_delivery
    ADD COLUMN IF NOT EXISTS to_email TEXT NULL;

CREATE INDEX IF NOT EXISTS idx_notification_delivery_to_email
    ON notification_delivery(to_email);
