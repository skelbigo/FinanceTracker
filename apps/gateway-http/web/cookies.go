package web

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	AccessCookie           = "access_token"
	RefreshCookie          = "refresh_token"
	CurrentWorkspaceCookie = "current_workspace"
)

const currentWorkspaceCookieMaxAgeDays = 180

type CookieConfig struct {
	Domain string
	Secure bool
}

func setAuthCookies(
	c *gin.Context,
	cfg CookieConfig,
	access string,
	accessTTL time.Duration,
	refresh string,
	refreshTTL time.Duration,
) {
	now := time.Now()

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     AccessCookie,
		Value:    access,
		Path:     "/",
		Domain:   cfg.Domain,
		MaxAge:   int(accessTTL.Seconds()),
		Expires:  now.Add(accessTTL),
		Secure:   cfg.Secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     RefreshCookie,
		Value:    refresh,
		Path:     "/",
		Domain:   cfg.Domain,
		MaxAge:   int(refreshTTL.Seconds()),
		Expires:  now.Add(refreshTTL),
		Secure:   cfg.Secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearAuthCookies(c *gin.Context, cfg CookieConfig) {
	exp := time.Unix(0, 0)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     AccessCookie,
		Value:    "",
		Path:     "/",
		Domain:   cfg.Domain,
		MaxAge:   -1,
		Expires:  exp,
		Secure:   cfg.Secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     RefreshCookie,
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

func setCurrentWorkspaceCookie(c *gin.Context, cfg CookieConfig, workspaceID string) {
	maxAge := int((currentWorkspaceCookieMaxAgeDays * 24 * time.Hour).Seconds())
	exp := time.Now().Add(time.Duration(currentWorkspaceCookieMaxAgeDays) * 24 * time.Hour)

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CurrentWorkspaceCookie,
		Value:    workspaceID,
		Path:     "/",
		Domain:   cfg.Domain,
		MaxAge:   maxAge,
		Expires:  exp,
		Secure:   cfg.Secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearCurrentWorkspaceCookie(c *gin.Context, cfg CookieConfig) {
	exp := time.Unix(0, 0)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CurrentWorkspaceCookie,
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
