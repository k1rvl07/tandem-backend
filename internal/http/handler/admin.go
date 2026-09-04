package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/http/dto"
	"github.com/tandem/tandem/internal/http/middleware"
	"github.com/tandem/tandem/internal/usecase/admin"
)

type AdminHandler struct {
	admin admin.UseCase
}

func NewAdminHandler(uc admin.UseCase) *AdminHandler {
	return &AdminHandler{admin: uc}
}

// CreateUser creates a corporate employee account.
// @Summary Create a user
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateUserRequest true "Employee account payload"
// @Success 201 {object} dto.UserResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/v1/admin/users [post]
func (h *AdminHandler) CreateUser(c *gin.Context) {
	var req dto.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	actor := admin.Actor{ID: currentUserID(c), Role: c.GetString(middleware.CtxUserRole)}
	resp, err := h.admin.CreateUser(c.Request.Context(), actor, req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// ListUsers lists all accounts.
// @Summary List users
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Success 200 {array} dto.UserResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /api/v1/admin/users [get]
func (h *AdminHandler) ListUsers(c *gin.Context) {
	users, err := h.admin.ListUsers(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, users)
}

// DeleteUser removes an employee account.
// @Summary Delete a user
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/admin/users/{id} [delete]
func (h *AdminHandler) DeleteUser(c *gin.Context) {
	actor := admin.Actor{ID: currentUserID(c), Role: c.GetString(middleware.CtxUserRole)}
	if err := h.admin.DeleteUser(c.Request.Context(), actor, c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "user deleted"})
}
