package cryptox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	PayloadVersion = 1
	ivSize         = 12
	authTagSize    = 16
)

type Payload struct {
	V   int    `json:"v"`
	KID string `json:"kid,omitempty"`
	IV  string `json:"iv"`
	CT  string `json:"ct"`
	Tag string `json:"tag"`
}

type AESGCM struct {
	key   []byte
	keyID string
}

func NewAESGCM(key []byte, keyID string) (*AESGCM, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("AES-256-GCM requires 32-byte key (got %d)", len(key))
	}
	if keyID == "" {
		keyID = "k1"
	}
	k := make([]byte, 32)
	copy(k, key)
	return &AESGCM{key: k, keyID: keyID}, nil
}

func (a *AESGCM) EncryptToPayload(plaintext []byte) (Payload, error) {
	block, err := aes.NewCipher(a.key)
	if err != nil {
		return Payload{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return Payload{}, err
	}

	iv := make([]byte, ivSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return Payload{}, err
	}

	sealed := gcm.Seal(nil, iv, plaintext, nil)
	if len(sealed) < authTagSize {
		return Payload{}, errors.New("unexpected gcm output")
	}
	ct := sealed[:len(sealed)-authTagSize]
	tag := sealed[len(sealed)-authTagSize:]

	return Payload{
		V:   PayloadVersion,
		KID: a.keyID,
		IV:  base64.StdEncoding.EncodeToString(iv),
		CT:  base64.StdEncoding.EncodeToString(ct),
		Tag: base64.StdEncoding.EncodeToString(tag),
	}, nil
}

func (a *AESGCM) DecryptPayload(p Payload) ([]byte, error) {
	if p.V != PayloadVersion {
		return nil, fmt.Errorf("unsupported payload version: %d", p.V)
	}

	iv, err := base64.StdEncoding.DecodeString(p.IV)
	if err != nil {
		return nil, errors.New("invalid payload iv")
	}
	ct, err := base64.StdEncoding.DecodeString(p.CT)
	if err != nil {
		return nil, errors.New("invalid payload ciphertext")
	}
	tag, err := base64.StdEncoding.DecodeString(p.Tag)
	if err != nil {
		return nil, errors.New("invalid payload tag")
	}
	sealed := append(ct, tag...)

	block, err := aes.NewCipher(a.key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	pt, err := gcm.Open(nil, iv, sealed, nil)
	if err != nil {
		return nil, err
	}
	return pt, nil
}

func (a *AESGCM) EncryptString(plaintext string) (string, error) {
	p, err := a.EncryptToPayload([]byte(plaintext))
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (a *AESGCM) DecryptString(payloadJSON string) (string, error) {
	var p Payload
	if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
		return "", err
	}
	pt, err := a.DecryptPayload(p)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}
