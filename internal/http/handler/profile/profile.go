package profile

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/http/dto/profile"
	"github.com/tandem/tandem/internal/http/handler/common"
	mpauth "github.com/tandem/tandem/internal/http/mapping/auth"
	mpprofile "github.com/tandem/tandem/internal/http/mapping/profile"
	pprofile "github.com/tandem/tandem/internal/usecase/profile"
)

type ProfileHandler struct {
	profiles     pprofile.Usecase
	cookieSecure bool
	refreshTTL   time.Duration
}

const maxImageSize = 5 << 20

func NewProfileHandler(uc pprofile.Usecase, cookieSecure bool, refreshTTL time.Duration) *ProfileHandler {
	return &ProfileHandler{profiles: uc, cookieSecure: cookieSecure, refreshTTL: refreshTTL}
}

// @Summary Get current user's profile
// @Tags profile
// @Produce json
// @Security BearerAuth
// @Success 200 {object} auth.UserResponse
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/me [get]
func (h *ProfileHandler) GetProfile(c *gin.Context) {
	userID := common.CurrentUserID(c)
	resp, err := h.profiles.Get(c.Request.Context(), userID)
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, mpauth.ToWireUser(*resp))
}

// @Summary Update current user's profile
// @Tags profile
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body profile.UpdateProfileRequest true "Profile fields"
// @Success 200 {object} auth.UserResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/me [patch]
func (h *ProfileHandler) UpdateProfile(c *gin.Context) {
	var req profile.UpdateProfileRequest
	if !common.ParseJSON(c, &req) {
		return
	}
	userID := common.CurrentUserID(c)
	resp, err := h.profiles.UpdateProfile(c.Request.Context(), userID, mpprofile.ToContractUpdateProfile(req))
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, mpauth.ToWireUser(*resp))
}

// @Summary Upload current user's avatar
// @Tags profile
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "Avatar image"
// @Success 200 {object} auth.UserResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 413 {object} map[string]string
// @Router /api/v1/me/avatar [post]
func (h *ProfileHandler) UploadAvatar(c *gin.Context) {
	userID := common.CurrentUserID(c)
	file, header, ok := common.UploadFile(c, maxImageSize)
	if !ok {
		return
	}
	defer file.Close()

	resp, err := h.profiles.UploadAvatar(c.Request.Context(), userID, header.Filename, header.Header.Get("Content-Type"), file, header.Size)
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, mpauth.ToWireUser(*resp))
}

// @Summary Delete current user's avatar
// @Tags profile
// @Produce json
// @Security BearerAuth
// @Success 200 {object} auth.UserResponse
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/me/avatar [delete]
func (h *ProfileHandler) DeleteAvatar(c *gin.Context) {
	resp, err := h.profiles.RemoveAvatar(c.Request.Context(), common.CurrentUserID(c))
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, mpauth.ToWireUser(*resp))
}

// @Summary Change current user's password
// @Tags profile
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body profile.ChangePasswordRequest true "New password and current password"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/me/password [post]
func (h *ProfileHandler) ChangePassword(c *gin.Context) {
	var req profile.ChangePasswordRequest
	if !common.ParseJSON(c, &req) {
		return
	}
	userID := common.CurrentUserID(c)
	resp, err := h.profiles.ChangePassword(c.Request.Context(), userID, mpprofile.ToContractChangePassword(req))
	if err != nil {
		common.RespondError(c, err)
		return
	}
	common.SetRefreshCookie(c.Writer, resp.RefreshToken, h.refreshTTL, h.cookieSecure)
	c.JSON(http.StatusOK, gin.H{"token": resp.Token, "message": "password updated"})
}
