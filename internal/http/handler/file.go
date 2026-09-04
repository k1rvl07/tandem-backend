package handler

import (
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/http/middleware"
	fileuc "github.com/tandem/tandem/internal/usecase/file"
)

type FileHandler struct {
	files *fileuc.Service
}

func NewFileHandler(uc *fileuc.Service) *FileHandler {
	return &FileHandler{files: uc}
}

const maxImageSize = 5 << 20

type uploadResponse struct {
	Key string `json:"key"`
	URL string `json:"url"`
}

// UploadImage uploads an image file.
// @Summary Upload an image
// @Tags files
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Image file"
// @Param namespace query string false "Namespace"
// @Security BearerAuth
// @Success 201 {object} uploadResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 413 {object} map[string]string
// @Router /api/v1/files/images [post]
func (h *FileHandler) UploadImage(c *gin.Context) {
	userID, _ := c.Get(middleware.CtxUserID)
	uid, _ := userID.(string)
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
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

	namespace := c.DefaultQuery("namespace", "avatars")

	key, err := h.files.UploadImage(c.Request.Context(), uid, namespace, header.Filename, header.Header.Get("Content-Type"), file, header.Size)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, uploadResponse{Key: key, URL: "/api/v1/files/" + key})
}

// GetImage streams an image file publicly.
// @Summary Get an image
// @Tags files
// @Produce image/*
// @Param key path string true "File key"
// @Success 200 {file} binary
// @Failure 404 {object} map[string]string
// @Router /api/v1/files/{key} [get]
func (h *FileHandler) GetImage(c *gin.Context) {
	key := strings.TrimPrefix(c.Param("key"), "/")
	rc, contentType, err := h.files.Open(c.Request.Context(), key)
	if err != nil {
		respondError(c, err)
		return
	}
	defer rc.Close()

	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, rc)
}
