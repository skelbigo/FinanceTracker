package notificationsv1

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
)

const (
	ServiceName                  = "contracts.notifications.v1.NotificationService"
	FullMethodCreateNotification = "/" + ServiceName + "/CreateNotification"
)

type CreateNotificationRequest struct {
	WorkspaceId      string
	RecipientUserIds []string
	Type             string
	Title            string
	Body             string
	PayloadJson      string
}

func (m *CreateNotificationRequest) Marshal() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	var b []byte
	if m.WorkspaceId != "" {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendString(b, m.WorkspaceId)
	}
	for _, uid := range m.RecipientUserIds {
		if uid == "" {
			continue
		}
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendString(b, uid)
	}
	if m.Type != "" {
		b = protowire.AppendTag(b, 3, protowire.BytesType)
		b = protowire.AppendString(b, m.Type)
	}
	if m.Title != "" {
		b = protowire.AppendTag(b, 4, protowire.BytesType)
		b = protowire.AppendString(b, m.Title)
	}
	if m.Body != "" {
		b = protowire.AppendTag(b, 5, protowire.BytesType)
		b = protowire.AppendString(b, m.Body)
	}
	if m.PayloadJson != "" {
		b = protowire.AppendTag(b, 6, protowire.BytesType)
		b = protowire.AppendString(b, m.PayloadJson)
	}
	return b, nil
}

func (m *CreateNotificationRequest) Unmarshal(in []byte) error {
	if m == nil {
		return fmt.Errorf("nil CreateNotificationRequest")
	}
	m.RecipientUserIds = m.RecipientUserIds[:0]

	for len(in) > 0 {
		num, typ, n := protowire.ConsumeTag(in)
		if n < 0 {
			return fmt.Errorf("invalid protobuf tag")
		}
		in = in[n:]

		switch num {
		case 1, 2, 3, 4, 5, 6:
			if typ != protowire.BytesType {
				n := protowire.ConsumeFieldValue(num, typ, in)
				if n < 0 {
					return fmt.Errorf("invalid field value")
				}
				in = in[n:]
				continue
			}
			v, n := protowire.ConsumeBytes(in)
			if n < 0 {
				return fmt.Errorf("invalid bytes value")
			}
			in = in[n:]

			s := string(v)
			switch num {
			case 1:
				m.WorkspaceId = s
			case 2:
				m.RecipientUserIds = append(m.RecipientUserIds, s)
			case 3:
				m.Type = s
			case 4:
				m.Title = s
			case 5:
				m.Body = s
			case 6:
				m.PayloadJson = s
			}
		default:
			n := protowire.ConsumeFieldValue(num, typ, in)
			if n < 0 {
				return fmt.Errorf("invalid unknown field")
			}
			in = in[n:]
		}
	}

	return nil
}

type CreateNotificationResponse struct {
	NotificationIds []string
}

func (m *CreateNotificationResponse) Marshal() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	var b []byte
	for _, id := range m.NotificationIds {
		if id == "" {
			continue
		}
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendString(b, id)
	}
	return b, nil
}

func (m *CreateNotificationResponse) Unmarshal(in []byte) error {
	if m == nil {
		return fmt.Errorf("nil CreateNotificationResponse")
	}
	m.NotificationIds = m.NotificationIds[:0]
	for len(in) > 0 {
		num, typ, n := protowire.ConsumeTag(in)
		if n < 0 {
			return fmt.Errorf("invalid protobuf tag")
		}
		in = in[n:]

		if num != 1 || typ != protowire.BytesType {
			n := protowire.ConsumeFieldValue(num, typ, in)
			if n < 0 {
				return fmt.Errorf("invalid field value")
			}
			in = in[n:]
			continue
		}

		v, n := protowire.ConsumeBytes(in)
		if n < 0 {
			return fmt.Errorf("invalid bytes value")
		}
		in = in[n:]
		m.NotificationIds = append(m.NotificationIds, string(v))
	}
	return nil
}
