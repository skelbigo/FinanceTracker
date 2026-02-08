package notifications

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/skelbigo/FinanceTracker/packages/contracts/notificationsv1"
	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/grpcx"
)

func RegisterGRPC(mux *http.ServeMux, svc *Service, logger *log.Logger) {
	if mux == nil {
		panic("notifications: nil mux")
	}
	if svc == nil {
		panic("notifications: nil service")
	}
	if logger == nil {
		logger = log.Default()
	}

	mux.HandleFunc(notificationsv1.FullMethodCreateNotification, func(w http.ResponseWriter, r *http.Request) {
		grpcx.EnsureTrailers(w)

		if r.Method != http.MethodPost {
			grpcx.WriteErrorTrailers(w, 12, "method not allowed")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if ct := r.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, grpcx.ContentType) {
			grpcx.WriteErrorTrailers(w, 3, "invalid content-type")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		in, err := grpcx.ReadUnaryMessage(r.Body)
		if err != nil {
			grpcx.WriteErrorTrailers(w, 13, "failed to read request")
			w.WriteHeader(http.StatusOK)
			return
		}

		var req notificationsv1.CreateNotificationRequest
		if err := req.Unmarshal(in); err != nil {
			grpcx.WriteErrorTrailers(w, 3, "invalid protobuf")
			w.WriteHeader(http.StatusOK)
			return
		}

		if strings.TrimSpace(req.WorkspaceId) == "" {
			grpcx.WriteErrorTrailers(w, 3, "workspace_id is required")
			w.WriteHeader(http.StatusOK)
			return
		}
		if len(req.RecipientUserIds) == 0 {
			grpcx.WriteErrorTrailers(w, 3, "recipient_user_ids is required")
			w.WriteHeader(http.StatusOK)
			return
		}

		payload := map[string]any{}
		if strings.TrimSpace(req.PayloadJson) != "" {
			if err := json.Unmarshal([]byte(req.PayloadJson), &payload); err != nil {
				grpcx.WriteErrorTrailers(w, 3, "invalid payload_json")
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		ws := strings.TrimSpace(req.WorkspaceId)
		typ := NotificationType(strings.TrimSpace(req.Type))
		if typ == "" {
			typ = TypeOverspending
		}
		title := strings.TrimSpace(req.Title)
		if title == "" {
			title = "Notification"
		}
		body := strings.TrimSpace(req.Body)

		ids := make([]string, 0, len(req.RecipientUserIds))
		for _, uid := range req.RecipientUserIds {
			uid = strings.TrimSpace(uid)
			if uid == "" {
				continue
			}
			n, err := svc.CreateInApp(r.Context(), uid, &ws, typ, title, body, payload)
			if err != nil {
				logger.Printf("notifications grpc CreateNotification: user=%s err=%v", uid, err)
				grpcx.WriteErrorTrailers(w, 13, "failed to create notification")
				w.WriteHeader(http.StatusOK)
				return
			}
			ids = append(ids, n.ID.String())
		}

		outMsg, err := (&notificationsv1.CreateNotificationResponse{NotificationIds: ids}).Marshal()
		if err != nil {
			grpcx.WriteErrorTrailers(w, 13, "failed to marshal response")
			w.WriteHeader(http.StatusOK)
			return
		}

		grpcx.WriteOKTrailers(w)
		w.WriteHeader(http.StatusOK)
		if err := grpcx.WriteUnaryMessage(w, outMsg); err != nil {
			logger.Printf("notifications grpc response write: %v", err)
		}
	})
}
