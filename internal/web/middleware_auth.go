package web

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/skelbigo/FinanceTracker/internal/auth"
)

func RequireAuth(
	jwtm *auth.JWTManager,
	authSvc *auth.Service,
	cfg CookieConfig,
	accessTTL, refreshTTL time.Duration,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		access, err := c.Cookie(AccessCookie)
		if err == nil && access != "" {
			userID, verr := jwtm.Verify(access)
			if verr == nil {
				c.Set(auth.CtxUserIDKey, userID)
				c.Next()
				return
			}

			if verr != auth.ErrExpiredToken {
				clearAuthCookies(c, cfg)
				c.Redirect(http.StatusSeeOther, "/login?flash=Please+login")
				c.Abort()
				return
			}
		}

		refresh, rerr := c.Cookie(RefreshCookie)
		if rerr != nil || refresh == "" {
			clearAuthCookies(c, cfg)
			c.Redirect(http.StatusSeeOther, "/login?flash=Please+login")
			c.Abort()
			return
		}

		out, err := authSvc.Refresh(c.Request.Context(), auth.RefreshRequest{RefreshToken: refresh})
		if err != nil {
			clearAuthCookies(c, cfg)
			c.Redirect(http.StatusSeeOther, "/login?flash=Please+login")
			c.Abort()
			return
		}

		setAuthCookies(c, cfg, out.AccessToken, accessTTL, out.RefreshToken, refreshTTL)

		userID, verr := jwtm.Verify(out.AccessToken)
		if verr != nil {
			clearAuthCookies(c, cfg)
			c.Redirect(http.StatusSeeOther, "/login?flash=Please+login")
			c.Abort()
			return
		}

		c.Set(auth.CtxUserIDKey, userID)
		c.Next()
	}
}
