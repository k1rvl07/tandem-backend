package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/http/dto/auth"
	"github.com/tandem/tandem/internal/http/handler/common"
	"github.com/tandem/tandem/internal/pkg/ctxkeys"
	pauth "github.com/tandem/tandem/internal/usecase/auth"
)

type AuthHandler struct {
	auth pauth.UseCase
}

func NewAuthHandler(uc pauth.UseCase) *AuthHandler {
	return &AuthHandler{auth: uc}
}

// @Summary Login a user
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.LoginRequest true "Login payload"
// @Success 200 {object} auth.LoginResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req auth.LoginRequest
	if !common.ParseJSON(c, &req) {
		return
	}
	resp, err := h.auth.Login(c.Request.Context(), req)
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Refresh access token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.RefreshRequest true "Refresh payload"
// @Success 200 {object} auth.RefreshResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req auth.RefreshRequest
	if !common.ParseJSON(c, &req) {
		return
	}
	resp, err := h.auth.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Log out current user
// @Tags auth
// @Produce json
// @Success 204
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	userID := c.GetString(ctxkeys.CtxUserID)
	if err := h.auth.Logout(c.Request.Context(), userID); err != nil {
		common.RespondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
