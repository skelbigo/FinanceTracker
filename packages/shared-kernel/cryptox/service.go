package cryptox

import (
	"encoding/json"
	"errors"
	"fmt"
)

var ErrDecryptFailed = errors.New("decrypt failed")

type Service struct {
	enabled bool

	activeKID string
	encryptor *AESGCM
	decryptor map[string]*AESGCM
}

func NewService(enabled bool, key []byte, keyID string) (*Service, error) {
	keys := map[string][]byte{}
	if len(key) > 0 {
		if keyID == "" {
			keyID = "k1"
		}
		keys[keyID] = key
	}
	return NewServiceWithKeyring(enabled, keyID, keys)
}

func NewServiceWithKeyring(enabled bool, activeKID string, keys map[string][]byte) (*Service, error) {
	s := &Service{enabled: enabled, activeKID: activeKID, decryptor: map[string]*AESGCM{}}
	if keys == nil {
		keys = map[string][]byte{}
	}
	for kid, k := range keys {
		if len(k) == 0 {
			continue
		}
		a, err := NewAESGCM(k, kid)
		if err != nil {
			return nil, err
		}
		s.decryptor[kid] = a
	}
	if ak, ok := s.decryptor[activeKID]; ok {
		s.encryptor = ak
	}
	if enabled && s.encryptor == nil {
		return nil, errors.New("encryption enabled but active key is not configured")
	}
	return s, nil
}

func (s *Service) Enabled() bool { return s != nil && s.enabled }

func (s *Service) canDecrypt() bool { return s != nil && len(s.decryptor) > 0 }

func (s *Service) decryptBytes(p Payload) ([]byte, error) {
	if !s.canDecrypt() {
		return nil, errors.New("encryption key is not configured")
	}

	if p.KID != "" {
		a := s.decryptor[p.KID]
		if a == nil {
			return nil, fmt.Errorf("no key for kid %q", p.KID)
		}
		return a.DecryptPayload(p)
	}

	tried := map[string]bool{}
	if s.activeKID != "" {
		if a := s.decryptor[s.activeKID]; a != nil {
			tried[s.activeKID] = true
			if pt, err := a.DecryptPayload(p); err == nil {
				return pt, nil
			}
		}
	}
	for kid, a := range s.decryptor {
		if tried[kid] {
			continue
		}
		if pt, err := a.DecryptPayload(p); err == nil {
			return pt, nil
		}
	}
	return nil, errors.New("decrypt failed for all keys")
}

func (s *Service) EncryptToPayload(plaintext string) (Payload, error) {
	if !s.Enabled() {
		return Payload{}, errors.New("encryption disabled")
	}
	return s.encryptor.EncryptToPayload([]byte(plaintext))
}

func (s *Service) DecryptPayload(p Payload) (string, error) {
	pt, err := s.decryptBytes(p)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecryptFailed, err)
	}
	return string(pt), nil
}

func (s *Service) EncryptForStorage(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if !s.Enabled() {
		return plaintext, nil
	}
	p, err := s.encryptor.EncryptToPayload([]byte(plaintext))
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *Service) DecryptFromStorage(stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	if !s.canDecrypt() {
		// No key configured; treat as pass-through.
		return stored, nil
	}

	var p Payload
	if err := json.Unmarshal([]byte(stored), &p); err != nil {
		if s.Enabled() {
			return "", fmt.Errorf("%w: invalid payload", ErrDecryptFailed)
		}
		return stored, nil
	}
	if p.V != PayloadVersion || p.IV == "" || p.CT == "" || p.Tag == "" {
		if s.Enabled() {
			return "", fmt.Errorf("%w: invalid payload", ErrDecryptFailed)
		}
		return stored, nil
	}

	pt, err := s.decryptBytes(p)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecryptFailed, err)
	}
	return string(pt), nil
}
