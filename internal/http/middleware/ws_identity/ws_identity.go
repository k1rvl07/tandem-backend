package ws_identity

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/domain/ports/service"
	"github.com/tandem/tandem/internal/pkg/ctxkeys"
)

func WSIdentity(tokens service.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := tokenFromSubprotocol(c.GetHeader("Sec-WebSocket-Protocol"))
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		userID, err := tokens.Parse(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Set(ctxkeys.CtxUserID, userID)
		c.Next()
	}
}

func tokenFromSubprotocol(header string) string {
	entries := strings.Split(header, ",")
	foundTandem := false
	for _, entry := range entries {
		if strings.TrimSpace(entry) == "tandem" {
			foundTandem = true
			break
		}
	}
	if !foundTandem {
		return ""
	}
	for _, entry := range entries {
		if token := strings.TrimSpace(entry); token != "" && token != "tandem" {
			return token
		}
	}
	return ""
}
