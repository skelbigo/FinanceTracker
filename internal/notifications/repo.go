package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

type CreateParams struct {
	UserID      uuid.UUID
	WorkspaceID *uuid.UUID
	Type        NotificationType
	Title       string
	Body        string
	Payload     map[string]any
}

func (r *Repo) Create(ctx context.Context, p CreateParams) (Notification, error) {
	if p.Payload == nil {
		p.Payload = map[string]any{}
	}
	payloadB, err := json.Marshal(p.Payload)
	if err != nil {
		return Notification{}, err
	}

	const q = `
INSERT INTO notifications (user_id, workspace_id, type, title, body, payload)
VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6::jsonb)
RETURNING id, user_id, workspace_id, type, title, body, payload, is_read, created_at;
`

	var out Notification
	var payloadRaw []byte
	var wsID *uuid.UUID
	var typ string

	err = r.pool.QueryRow(ctx, q, p.UserID, p.WorkspaceID, string(p.Type), p.Title, p.Body, payloadB).
		Scan(&out.ID, &out.UserID, &wsID, &typ, &out.Title, &out.Body, &payloadRaw, &out.IsRead, &out.CreatedAt)
	if err != nil {
		return Notification{}, err
	}
	out.WorkspaceID = wsID
	out.Type = NotificationType(typ)
	if len(payloadRaw) > 0 {
		_ = json.Unmarshal(payloadRaw, &out.Payload)
	}
	if out.Payload == nil {
		out.Payload = map[string]any{}
	}
	return out, nil
}

func (r *Repo) ListForUser(ctx context.Context, userID uuid.UUID, onlyUnread bool, limit, offset int) ([]Notification, bool, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	queryLimit := limit + 1

	args := []any{userID, queryLimit, offset}
	where := "WHERE user_id = $1::uuid"
	if onlyUnread {
		where += " AND is_read = false"
	}

	q := fmt.Sprintf(`
SELECT id, user_id, workspace_id, type, title, body, payload, is_read, created_at
FROM notifications
%s
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;
`, where)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	out := make([]Notification, 0)
	for rows.Next() {
		var n Notification
		var payloadRaw []byte
		var wsID *uuid.UUID
		var typ string
		if err := rows.Scan(&n.ID, &n.UserID, &wsID, &typ, &n.Title, &n.Body, &payloadRaw, &n.IsRead, &n.CreatedAt); err != nil {
			return nil, false, err
		}
		n.WorkspaceID = wsID
		n.Type = NotificationType(typ)
		if len(payloadRaw) > 0 {
			_ = json.Unmarshal(payloadRaw, &n.Payload)
		}
		if n.Payload == nil {
			n.Payload = map[string]any{}
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	hasNext := false
	if len(out) > limit {
		hasNext = true
		out = out[:limit]
	}
	return out, hasNext, nil
}

func (r *Repo) CountUnread(ctx context.Context, userID uuid.UUID) (int, error) {
	const q = `SELECT COUNT(1) FROM notifications WHERE user_id=$1::uuid AND is_read=false;`
	var cnt int
	if err := r.pool.QueryRow(ctx, q, userID).Scan(&cnt); err != nil {
		return 0, err
	}
	return cnt, nil
}

func (r *Repo) MarkRead(ctx context.Context, userID, notificationID uuid.UUID) (bool, error) {
	const q = `UPDATE notifications SET is_read=true WHERE id=$1::uuid AND user_id=$2::uuid AND is_read=false;`
	ct, err := r.pool.Exec(ctx, q, notificationID, userID)
	if err != nil {
		return false, err
	}
	return ct.RowsAffected() > 0, nil
}

func (r *Repo) MarkAllRead(ctx context.Context, userID uuid.UUID) (int64, error) {
	const q = `UPDATE notifications SET is_read=true WHERE user_id=$1::uuid AND is_read=false;`
	ct, err := r.pool.Exec(ctx, q, userID)
	if err != nil {
		return 0, err
	}
	return ct.RowsAffected(), nil
}

func (r *Repo) GetForUser(ctx context.Context, userID, notificationID uuid.UUID) (Notification, bool, error) {
	const q = `
SELECT id, user_id, workspace_id, type, title, body, payload, is_read, created_at
FROM notifications
WHERE id=$1::uuid AND user_id=$2::uuid;
`
	var n Notification
	var payloadRaw []byte
	var wsID *uuid.UUID
	var typ string
	err := r.pool.QueryRow(ctx, q, notificationID, userID).
		Scan(&n.ID, &n.UserID, &wsID, &typ, &n.Title, &n.Body, &payloadRaw, &n.IsRead, &n.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Notification{}, false, nil
		}
		return Notification{}, false, err
	}
	n.WorkspaceID = wsID
	n.Type = NotificationType(typ)
	if len(payloadRaw) > 0 {
		_ = json.Unmarshal(payloadRaw, &n.Payload)
	}
	if n.Payload == nil {
		n.Payload = map[string]any{}
	}
	return n, true, nil
}

func (r *Repo) InsertDelivery(ctx context.Context, notificationID uuid.UUID, channel DeliveryChannel, status DeliveryStatus, errText *string, attempts int, sentAt *time.Time) error {
	const q = `
INSERT INTO notification_delivery (notification_id, channel, status, error, attempts, sent_at)
VALUES ($1::uuid, $2, $3, $4, $5, $6)
ON CONFLICT (notification_id, channel) DO UPDATE
SET
	status = EXCLUDED.status,
	error = EXCLUDED.error,
	attempts = notification_delivery.attempts + EXCLUDED.attempts,
	sent_at = EXCLUDED.sent_at,
	updated_at = now();
`
	_, err := r.pool.Exec(ctx, q, notificationID, string(channel), string(status), errText, attempts, sentAt)
	return err
}

func (r *Repo) EnsureDelivery(ctx context.Context, notificationID uuid.UUID, channel DeliveryChannel, status DeliveryStatus) error {
	const q = `
INSERT INTO notification_delivery (notification_id, channel, status)
VALUES ($1::uuid, $2, $3)
ON CONFLICT (notification_id, channel) DO NOTHING;
`
	_, err := r.pool.Exec(ctx, q, notificationID, string(channel), string(status))
	return err
}

type EmailDeliveryJob struct {
	DeliveryID uuid.UUID
	ToEmail    string
	Notif      Notification
}

func (r *Repo) ClaimPendingEmailDeliveries(ctx context.Context, batchSize int, maxAttempts int) ([]EmailDeliveryJob, error) {
	if batchSize <= 0 {
		batchSize = 20
	}
	if batchSize > 200 {
		batchSize = 200
	}
	if maxAttempts <= 0 {
		maxAttempts = 5
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const q = `
SELECT d.id,
       u.email,
       n.id, n.user_id, n.workspace_id, n.type, n.title, n.body, n.payload, n.is_read, n.created_at
FROM notification_delivery d
JOIN notifications n ON n.id = d.notification_id
JOIN users u ON u.id = n.user_id
WHERE d.channel = 'email'
  AND d.attempts < $1
  AND (
    d.status IN ('pending', 'failed')
    OR (d.status = 'processing' AND d.updated_at < (now() - interval '5 minutes'))
  )
ORDER BY d.created_at ASC
FOR UPDATE SKIP LOCKED
LIMIT $2;
`
	rows, err := tx.Query(ctx, q, maxAttempts, batchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]EmailDeliveryJob, 0)
	for rows.Next() {
		var job EmailDeliveryJob
		var n Notification
		var payloadRaw []byte
		var wsID *uuid.UUID
		var typ string
		if err := rows.Scan(&job.DeliveryID, &job.ToEmail, &n.ID, &n.UserID, &wsID, &typ, &n.Title, &n.Body, &payloadRaw, &n.IsRead, &n.CreatedAt); err != nil {
			return nil, err
		}
		n.WorkspaceID = wsID
		n.Type = NotificationType(typ)
		if len(payloadRaw) > 0 {
			_ = json.Unmarshal(payloadRaw, &n.Payload)
		}
		if n.Payload == nil {
			n.Payload = map[string]any{}
		}
		job.Notif = n
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	const uq = `
UPDATE notification_delivery
SET status = $2,
    attempts = attempts + 1,
    error = NULL,
    updated_at = now()
WHERE id = $1::uuid;
`
	for _, j := range jobs {
		if _, err := tx.Exec(ctx, uq, j.DeliveryID, string(StatusProcessing)); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (r *Repo) UpdateDeliveryByID(ctx context.Context, deliveryID uuid.UUID, status DeliveryStatus, errText *string, sentAt *time.Time) error {
	const q = `
UPDATE notification_delivery
SET status = $2,
    error = $3,
    sent_at = $4,
    updated_at = now()
WHERE id = $1::uuid;
`
	_, err := r.pool.Exec(ctx, q, deliveryID, string(status), errText, sentAt)
	return err
}

func (r *Repo) UpsertPushSubscription(ctx context.Context, userID uuid.UUID, endpoint string, keys map[string]any) (PushSubscription, error) {
	if keys == nil {
		keys = map[string]any{}
	}
	keysB, err := json.Marshal(keys)
	if err != nil {
		return PushSubscription{}, err
	}
	const q = `
INSERT INTO push_subscriptions (user_id, endpoint, keys)
VALUES ($1::uuid, $2, $3::jsonb)
ON CONFLICT (user_id, endpoint) DO UPDATE SET keys=EXCLUDED.keys
RETURNING id, user_id, endpoint, keys, created_at;
`
	var out PushSubscription
	var keysRaw []byte
	err = r.pool.QueryRow(ctx, q, userID, endpoint, keysB).Scan(&out.ID, &out.UserID, &out.Endpoint, &keysRaw, &out.CreatedAt)
	if err != nil {
		return PushSubscription{}, err
	}
	_ = json.Unmarshal(keysRaw, &out.Keys)
	if out.Keys == nil {
		out.Keys = map[string]any{}
	}
	return out, nil
}

func (r *Repo) DeletePushSubscription(ctx context.Context, userID uuid.UUID, endpoint string) (bool, error) {
	const q = `DELETE FROM push_subscriptions WHERE user_id=$1::uuid AND endpoint=$2;`
	ct, err := r.pool.Exec(ctx, q, userID, endpoint)
	if err != nil {
		return false, err
	}
	return ct.RowsAffected() > 0, nil
}
