CREATE TABLE IF NOT EXISTS notifications (
                                             id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id UUID NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
    );

CREATE INDEX IF NOT EXISTS idx_notifications_user_read_created
    ON notifications(user_id, is_read, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_notifications_workspace_created
    ON notifications(workspace_id, created_at DESC);

CREATE TABLE IF NOT EXISTS notification_delivery (
                                                     id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id UUID NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    channel TEXT NOT NULL,
    status TEXT NOT NULL,
    error TEXT NULL,
    attempts INT NOT NULL DEFAULT 0,
    sent_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
    );

CREATE INDEX IF NOT EXISTS idx_notification_delivery_notification_id
    ON notification_delivery(notification_id);

CREATE TABLE IF NOT EXISTS push_subscriptions (
                                                  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint TEXT NOT NULL,
    keys JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, endpoint)
    );

CREATE INDEX IF NOT EXISTS idx_push_subscriptions_user_id
    ON push_subscriptions(user_id);
