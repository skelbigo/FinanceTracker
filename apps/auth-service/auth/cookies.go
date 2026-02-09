package auth

import (
	"net/http"
	"time"
)

const (
	RefreshTokenCookie = "refresh_token"
)

type CookieConfig struct {
	Domain string
	Secure bool
}

func setRefreshCookie(w http.ResponseWriter, cfg CookieConfig, refreshToken string, ttl time.Duration) {
	now := time.Now()
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshTokenCookie,
		Value:    refreshToken,
		Path:     "/",
		Domain:   cfg.Domain,
		MaxAge:   int(ttl.Seconds()),
		Expires:  now.Add(ttl),
		Secure:   cfg.Secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearRefreshCookie(w http.ResponseWriter, cfg CookieConfig) {
	exp := time.Unix(0, 0)
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshTokenCookie,
		Value:    "",
		Path:     "/",
		Domain:   cfg.Domain,
		MaxAge:   -1,
		Expires:  exp,
		Secure:   cfg.Secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}
