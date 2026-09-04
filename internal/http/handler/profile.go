package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/http/dto"
	"github.com/tandem/tandem/internal/http/middleware"
	"github.com/tandem/tandem/internal/usecase/profile"
)

type ProfileHandler struct {
	profiles profile.UserCase
}

func NewProfileHandler(uc profile.UserCase) *ProfileHandler {
	return &ProfileHandler{profiles: uc}
}

// GetProfile returns the current user's profile.
// @Summary Get current user's profile
// @Tags profile
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.UserResponse
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/me [get]
func (h *ProfileHandler) GetProfile(c *gin.Context) {
	userID := currentUserID(c)
	resp, err := h.profiles.Get(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateProfile updates the current user's profile fields.
// @Summary Update current user's profile
// @Tags profile
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.UpdateProfileRequest true "Profile fields"
// @Success 200 {object} dto.UserResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/me [patch]
func (h *ProfileHandler) UpdateProfile(c *gin.Context) {
	var req dto.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	userID := currentUserID(c)
	resp, err := h.profiles.UpdateProfile(c.Request.Context(), userID, req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// UploadAvatar replaces the current user's avatar.
// @Summary Upload current user's avatar
// @Tags profile
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "Avatar image"
// @Success 200 {object} dto.UserResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 413 {object} map[string]string
// @Router /api/v1/me/avatar [post]
func (h *ProfileHandler) UploadAvatar(c *gin.Context) {
	userID := currentUserID(c)
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImageSize)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	resp, err := h.profiles.UploadAvatar(c.Request.Context(), userID, header.Filename, header.Header.Get("Content-Type"), file, header.Size)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ChangePassword changes the current user's password.
// @Summary Change current user's password
// @Tags profile
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.ChangePasswordRequest true "New password and current password"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/me/password [post]
func (h *ProfileHandler) ChangePassword(c *gin.Context) {
	var req dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	userID := currentUserID(c)
	if err := h.profiles.ChangePassword(c.Request.Context(), userID, req); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "password updated"})
}

func currentUserID(c *gin.Context) string {
	userID, _ := c.Get(middleware.CtxUserID)
	uid, _ := userID.(string)
	return uid
}
