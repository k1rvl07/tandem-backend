package common

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/pkg/ctxkeys"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
)

func CurrentUserID(c *gin.Context) string {
	userID, _ := c.Get(ctxkeys.CtxUserID)
	uid, _ := userID.(string)
	return uid
}

func RespondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, pkgerrors.ErrValidation):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, pkgerrors.ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"error": "conflict"})
	case errors.Is(err, pkgerrors.ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
	case errors.Is(err, pkgerrors.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	case errors.Is(err, pkgerrors.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
