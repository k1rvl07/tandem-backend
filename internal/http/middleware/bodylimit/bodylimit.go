package bodylimit

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func LimitBody(max int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
			c.Next()
			return
		}
		if c.Request.Body == nil {
			c.Next()
			return
		}
		limited := &io.LimitedReader{R: c.Request.Body, N: max + 1}
		buf, err := io.ReadAll(limited)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large"})
			return
		}
		if int64(len(buf)) > max {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "request body too large"})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(buf))
		c.Next()
	}
}
