package notifications

import "log"

type PushProvider interface {
	Send(userID string, payload map[string]any) error
}

type PushSender = PushProvider

type NoopPushProvider struct{}

func (NoopPushProvider) Send(userID string, payload map[string]any) error {
	log.Printf("noop push: user=%s payload=%v", userID, payload)
	return nil
}

type NoopPushSender = NoopPushProvider
