package require_staff

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	muser "github.com/tandem/tandem/internal/domain/models/user"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/pkg/ctxkeys"
)

type fakeUsers struct {
	repository.UserRepository
	user *muser.User
	err  error
}

func (f *fakeUsers) FindByID(_ context.Context, id string) (*muser.User, error) {
	return f.user, f.err
}

func staffRouter(users repository.UserRepository, id string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/admin", func(c *gin.Context) {
		c.Set(ctxkeys.CtxUserID, id)
		c.Next()
	}, RequireStaff(users), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return r
}

func TestRequireStaffAdmin(t *testing.T) {
	r := staffRouter(&fakeUsers{user: &muser.User{Role: muser.RoleAdmin}}, "u1")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestRequireStaffMemberForbidden(t *testing.T) {
	r := staffRouter(&fakeUsers{user: &muser.User{Role: muser.RoleUser}}, "u1")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequireStaffMissingUser(t *testing.T) {
	r := staffRouter(&fakeUsers{}, "u1")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestRequireStaffNoIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/admin", RequireStaff(&fakeUsers{user: &muser.User{Role: muser.RoleAdmin}}), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRequireStaffSetsRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var gotRole string
	r.GET("/admin", func(c *gin.Context) {
		c.Set(ctxkeys.CtxUserID, "u1")
		c.Next()
	}, RequireStaff(&fakeUsers{user: &muser.User{Role: muser.RoleModerator}}), func(c *gin.Context) {
		gotRole = c.GetString(CtxUserRole)
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, muser.RoleModerator, gotRole)
}
