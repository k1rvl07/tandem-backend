package openapi

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/swaggo/swag/v2"
)

//go:embed index.html
var indexHTML string

//go:embed swagger-initializer.js
var initializerJS string

func Register(r *gin.Engine, path string) {
	r.GET(path+"/index.html", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(indexHTML))
	})
	r.GET(path+"/swagger-initializer.js", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/javascript", []byte(initializerJS))
	})
	r.GET(path+"/doc.json", func(c *gin.Context) {
		doc, err := swag.ReadDoc()
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(doc))
	})
	r.GET(path, func(c *gin.Context) {
		c.Redirect(http.StatusFound, path+"/index.html")
	})
}
