package file

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/http/handler/common"
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
	file, header, ok := common.UploadFile(c, maxImageSize)
	if !ok {
		return
	}
	defer file.Close()

	namespace := c.DefaultQuery("namespace", "avatars")

	key, err := h.files.UploadImage(c.Request.Context(), common.CurrentUserID(c), namespace, header.Filename, header.Header.Get("Content-Type"), file, header.Size)
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
	key := strings.TrimSpace(c.Query("key"))
	if err := h.files.DeleteImage(c.Request.Context(), common.CurrentUserID(c), key); err != nil {
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
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Security BearerAuth
// @Router /api/v1/files/sign [get]
func (h *FileHandler) Sign(c *gin.Context) {
	key := strings.TrimSpace(c.Query("key"))
	url, err := h.files.SignImage(c.Request.Context(), common.CurrentUserID(c), key)
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url})
}
