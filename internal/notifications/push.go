package notifications

import "log"

type PushSender interface {
	Send(userID string, payload map[string]any) error
	Enabled() bool
}

type NoopPushSender struct{}

func (NoopPushSender) Send(userID string, payload map[string]any) error {
	log.Printf("noop push: user=%s payload=%v", userID, payload)
	return nil
}
func (NoopPushSender) Enabled() bool { return false }
