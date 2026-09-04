package middleware

import "github.com/gin-gonic/gin"

func CORS(allowOrigins []string) gin.HandlerFunc {
	allowAll := contains(allowOrigins, "*")
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowed := origin
		if !allowAll && origin != "" && !contains(allowOrigins, origin) {
			allowed = ""
		}
		if allowed != "" {
			if allowAll {
				c.Header("Access-Control-Allow-Origin", "*")
			} else {
				c.Header("Access-Control-Allow-Origin", allowed)
				c.Header("Access-Control-Allow-Credentials", "true")
			}
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
