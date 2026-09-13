package common

import (
	"errors"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/pkg/ctxkeys"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
)

func CurrentUserID(c *gin.Context) string {
	userID, _ := c.Get(ctxkeys.CtxUserID)
	uid, _ := userID.(string)
	return uid
}

func UploadFile(c *gin.Context, maxSize int64) (multipart.File, *multipart.FileHeader, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large"})
			return nil, nil, false
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return nil, nil, false
	}
	return file, header, true
}

func ParseJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return false
	}
	return true
}

func ParseQuery(c *gin.Context, dst any, invalidMsg string) bool {
	if err := c.ShouldBindQuery(dst); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": invalidMsg})
		return false
	}
	return true
}

type ErrorResponse struct {
	Error      string `json:"error"`
	Code       string `json:"code,omitempty"`
	RetryAfter int    `json:"retry_after,omitempty"`
}

func RespondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, pkgerrors.ErrValidation):
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	case errors.Is(err, pkgerrors.ErrConflict):
		c.JSON(http.StatusConflict, ErrorResponse{Error: "conflict"})
	case errors.Is(err, pkgerrors.ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "invalid credentials"})
	case errors.Is(err, pkgerrors.ErrNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "not found"})
	case errors.Is(err, pkgerrors.ErrForbidden):
		c.JSON(http.StatusForbidden, ErrorResponse{Error: "forbidden"})
	case errors.Is(err, pkgerrors.ErrTooManyRequests):
		c.JSON(http.StatusTooManyRequests, ErrorResponse{Error: err.Error()})
	case errors.Is(err, pkgerrors.ErrUnavailable):
		c.JSON(http.StatusServiceUnavailable, ErrorResponse{Error: err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
	}
}
