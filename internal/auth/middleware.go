package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/skelbigo/FinanceTracker/internal/httpx"
	"strings"
)

const CtxUserIDKey = "user_id"

func AuthRequired(jwtm *JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			httpx.Unauthorized(c, "invalid authorization")
			c.Abort()
			return
		}

		userID, err := jwtm.ParseAndValidate(tokenStr)
		if err != nil {
			httpx.Unauthorized(c, "invalid token")
			c.Abort()
			return
		}

		c.Set(CtxUserIDKey, userID)
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
