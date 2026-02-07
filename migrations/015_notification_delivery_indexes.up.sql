CREATE UNIQUE INDEX IF NOT EXISTS idx_notification_delivery_notification_channel
    ON notification_delivery(notification_id, channel);

CREATE INDEX IF NOT EXISTS idx_notification_delivery_status_created
    ON notification_delivery(status, created_at DESC);
