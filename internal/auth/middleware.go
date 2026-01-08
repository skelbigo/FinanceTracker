package auth

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

const CtxUserIDKey = "user_id"

func AuthRequired(jwtm *JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "invalid authorization"})
			return
		}

		userID, err := jwtm.ParseAndValidate(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "invalid token"})
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
