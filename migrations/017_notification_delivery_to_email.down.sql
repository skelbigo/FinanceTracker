DROP INDEX IF EXISTS idx_notification_delivery_to_email;

ALTER TABLE notification_delivery
DROP COLUMN IF EXISTS to_email;
