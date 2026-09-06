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

// @Summary List users
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number, starting from 1"
// @Param page_size query int false "Items per page, default 20, max 100"
// @Param q query string false "Search users by login substring"
// @Success 200 {object} dto.AdminPage
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /api/v1/admin/users [get]
func (h *AdminHandler) ListUsers(c *gin.Context) {
	var query dto.AdminListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query parameters"})
		return
	}
	page, err := h.admin.ListUsers(c.Request.Context(), currentUserID(c), query)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

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

// @Summary Update user role
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID"
// @Param request body dto.UpdateUserRoleRequest true "New role"
// @Success 200 {object} dto.UserResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/admin/users/{id}/role [patch]
func (h *AdminHandler) UpdateUserRole(c *gin.Context) {
	var req dto.UpdateUserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	actor := admin.Actor{ID: currentUserID(c), Role: c.GetString(middleware.CtxUserRole)}
	resp, err := h.admin.UpdateUserRole(c.Request.Context(), actor, c.Param("id"), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
