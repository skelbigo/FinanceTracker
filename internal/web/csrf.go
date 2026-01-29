package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func GenerateCSRF(secret string, ttl time.Duration) string {
	if secret == "" || ttl <= 0 {
		return ""
	}

	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return ""
	}

	exp := time.Now().Add(ttl).Unix()
	payload := base64.RawURLEncoding.EncodeToString([]byte(base64.RawURLEncoding.EncodeToString(nonce) + ":" + strconv.FormatInt(exp, 10)))

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	sig := mac.Sum(nil)

	return payload + "." + hex.EncodeToString(sig)
}

func ValidateCSRF(secret, token string) bool {
	if secret == "" {
		return false
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}

	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}
	payloadB64 := parts[0]
	sigHex := parts[1]

	gotSig, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payloadB64))
	wantSig := mac.Sum(nil)

	if !hmac.Equal(gotSig, wantSig) {
		return false
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return false
	}

	payload := string(payloadBytes)
	i := strings.LastIndexByte(payload, ':')
	if i <= 0 || i == len(payload)-1 {
		return false
	}

	expUnix, err := strconv.ParseInt(payload[i+1:], 10, 64)
	if err != nil {
		return false
	}

	return time.Now().Unix() < expUnix
}

func CSRFMiddleware(secrete string) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}

		token := c.GetHeader("X-CSRF-Token")

		if token == "" {
			ct := c.ContentType()
			if ct == "application/x-www-form-urlencoded" || ct == "multipart/form-data" {
				token = c.PostForm("csrf_token")
			}
		}

		if token == "" || !ValidateCSRF(secrete, token) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Next()
	}
}
