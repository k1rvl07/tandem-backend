package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/domain/models"
	"github.com/tandem/tandem/internal/domain/ports/repository"
)

const CtxUserRole = "userRole"

func RequireStaff(users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get(CtxUserID)
		uid, _ := userID.(string)
		if uid == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		user, err := users.FindByID(c.Request.Context(), uid)
		if err != nil || !models.StaffRoles[user.Role] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Set(CtxUserRole, user.Role)
		c.Next()
	}
}
