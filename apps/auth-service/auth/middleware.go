package auth

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/httpx"
	"github.com/skelbigo/FinanceTracker/packages/shared-kernel/identity"
)

const (
	accessTokenCookie = "access_token"
)

func AuthRequired(jwtm *JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, _ := bearerToken(c.GetHeader("Authorization"))

		if tokenStr == "" {
			if ck, err := c.Cookie(accessTokenCookie); err == nil && ck != "" {
				tokenStr = ck
			}
		}

		if tokenStr == "" {
			httpx.Unauthorized(c, "missing authorization")
			c.Abort()
			return
		}

		userID, err := jwtm.ParseAndValidate(tokenStr)
		if err != nil {
			httpx.Unauthorized(c, "invalid token")
			c.Abort()
			return
		}

		c.Set(identity.CtxUserIDKey, userID)
		c.Next()
	}
}

func bearerToken(h string) (string, bool) {
	h = strings.TrimSpace(h)
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	tok := strings.TrimSpace(parts[1])
	return tok, tok != ""
}
