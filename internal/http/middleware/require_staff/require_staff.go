package require_staff

import (
	"net/http"

	"github.com/gin-gonic/gin"
	muser "github.com/tandem/tandem/internal/domain/models/user"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/pkg/ctxkeys"
)

const CtxUserRole = "userRole"

func RequireStaff(users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get(ctxkeys.CtxUserID)
		uid, _ := userID.(string)
		if uid == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		user, err := users.FindByID(c.Request.Context(), uid)
		if err != nil || user == nil || !muser.StaffRoles[user.Role] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Set(CtxUserRole, user.Role)
		c.Next()
	}
}
