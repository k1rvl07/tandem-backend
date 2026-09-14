package openapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/swaggo/swag/v2"
)

type stubDoc struct{}

func (stubDoc) ReadDoc() string {
	return `{"openapi":"3.1.0","info":{"title":"Tandem","version":"1.0.0"}}`
}

func TestRegister(t *testing.T) {
	if swag.GetSwagger(swag.Name) == nil {
		swag.Register(swag.Name, stubDoc{})
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Register(r, "/swagger")
	srv := httptest.NewServer(r)
	defer srv.Close()

	tt := []struct {
		name string
		path string
		want int
	}{
		{name: "index", path: "/swagger/index.html", want: http.StatusOK},
		{name: "initializer", path: "/swagger/swagger-initializer.js", want: http.StatusOK},
		{name: "doc.json", path: "/swagger/doc.json", want: http.StatusOK},
		{name: "base redirect", path: "/swagger", want: http.StatusFound},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, srv.URL+tc.path, nil)
			require.NoError(t, err)
			client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, tc.want, resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			if tc.path == "/swagger/doc.json" {
				assert.JSONEq(t, stubDoc{}.ReadDoc(), string(body))
			}
			if tc.path == "/swagger" {
				assert.Equal(t, "/swagger/index.html", resp.Header.Get("Location"))
			}
		})
	}
}
