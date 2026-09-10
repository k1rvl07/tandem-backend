package file

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/http/handler/common"
	"github.com/tandem/tandem/internal/pkg/ctxkeys"
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
}

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
	userID, _ := c.Get(ctxkeys.CtxUserID)
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
		common.RespondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, uploadResponse{Key: key})
}

// @Summary Delete an uploaded image
// @Tags files
// @Param key query string true "File key"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Security BearerAuth
// @Router /api/v1/files/images [delete]
func (h *FileHandler) DeleteImage(c *gin.Context) {
	userID, _ := c.Get(ctxkeys.CtxUserID)
	uid, _ := userID.(string)
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	key := strings.TrimSpace(c.Query("key"))
	if err := h.files.DeleteImage(c.Request.Context(), uid, key); err != nil {
		common.RespondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Sign an image URL
// @Tags files
// @Produce json
// @Param key query string true "File key"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /api/v1/files/sign [get]
func (h *FileHandler) Sign(c *gin.Context) {
	key := strings.TrimSpace(c.Query("key"))
	url, err := h.files.SignImage(c.Request.Context(), key)
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url})
}
