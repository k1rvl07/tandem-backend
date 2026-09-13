package attachment

import (
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/http/handler/common"
	pattachment "github.com/tandem/tandem/internal/usecase/attachment"
)

type AttachmentHandler struct {
	attachments pattachment.UseCase
}

func NewAttachmentHandler(uc pattachment.UseCase) *AttachmentHandler {
	return &AttachmentHandler{attachments: uc}
}

const maxAttachmentRequestSize = 20 << 20

// @Summary Upload a task attachment
// @Tags attachments
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param id path string true "Workspace ID"
// @Param taskId path string true "Task ID"
// @Param file formData file true "Attachment file"
// @Success 201 {object} attachment.AttachmentResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 413 {object} map[string]string
// @Router /api/v1/workspaces/{id}/tasks/{taskId}/attachments [post]
func (h *AttachmentHandler) Create(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAttachmentRequestSize)
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
	resp, err := h.attachments.Create(
		c.Request.Context(),
		common.CurrentUserID(c),
		c.Param("id"),
		c.Param("taskId"),
		header.Filename,
		header.Header.Get("Content-Type"),
		file,
		header.Size,
	)
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// @Summary List task attachments
// @Tags attachments
// @Produce json
// @Security BearerAuth
// @Param id path string true "Workspace ID"
// @Param taskId path string true "Task ID"
// @Success 200 {array} attachment.AttachmentResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id}/tasks/{taskId}/attachments [get]
func (h *AttachmentHandler) List(c *gin.Context) {
	resp, err := h.attachments.List(c.Request.Context(), common.CurrentUserID(c), c.Param("id"), c.Param("taskId"))
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Download a task attachment
// @Tags attachments
// @Produce application/octet-stream
// @Security BearerAuth
// @Param id path string true "Workspace ID"
// @Param taskId path string true "Task ID"
// @Param attachmentId path string true "Attachment ID"
// @Success 200 {file} binary
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id}/tasks/{taskId}/attachments/{attachmentId} [get]
func (h *AttachmentHandler) Download(c *gin.Context) {
	resp, rc, err := h.attachments.Download(c.Request.Context(), common.CurrentUserID(c), c.Param("id"), c.Param("taskId"), c.Param("attachmentId"))
	if err != nil {
		common.RespondError(c, err)
		return
	}
	defer rc.Close()
	if resp.ContentType != "" {
		c.Header("Content-Type", resp.ContentType)
	} else {
		c.Header("Content-Type", "application/octet-stream")
	}
	c.Header("Content-Disposition", "attachment; filename=\""+sanitizeFilename(resp.Filename)+"\"")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, rc)
}

func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, "\\", "")
	name = strings.ReplaceAll(name, "\"", "")
	name = strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, name)
	return name
}

// @Summary Delete a task attachment
// @Tags attachments
// @Produce json
// @Security BearerAuth
// @Param id path string true "Workspace ID"
// @Param taskId path string true "Task ID"
// @Param attachmentId path string true "Attachment ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id}/tasks/{taskId}/attachments/{attachmentId} [delete]
func (h *AttachmentHandler) Delete(c *gin.Context) {
	if err := h.attachments.Delete(c.Request.Context(), common.CurrentUserID(c), c.Param("id"), c.Param("taskId"), c.Param("attachmentId")); err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "attachment deleted"})
}
