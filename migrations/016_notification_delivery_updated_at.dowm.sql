DROP INDEX IF EXISTS idx_notification_delivery_status_updated;

ALTER TABLE notification_delivery
DROP COLUMN IF EXISTS updated_at;
