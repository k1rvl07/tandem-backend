package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/http/dto"
	"github.com/tandem/tandem/internal/usecase/column"
)

type ColumnHandler struct {
	columns column.UseCase
}

func NewColumnHandler(uc column.UseCase) *ColumnHandler {
	return &ColumnHandler{columns: uc}
}

// Create appends a column to a board.
// @Summary Create a column
// @Tags columns
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param wsId path string true "Workspace ID"
// @Param boardId path string true "Board ID"
// @Param request body dto.CreateColumnRequest true "Column payload"
// @Success 201 {object} dto.ColumnDetailResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id}/boards/{boardId}/columns [post]
func (h *ColumnHandler) Create(c *gin.Context) {
	var req dto.CreateColumnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	resp, err := h.columns.Create(c.Request.Context(), currentUserID(c), c.Param("id"), c.Param("boardId"), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// Update renames or reorders a column.
// @Summary Update a column
// @Tags columns
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param wsId path string true "Workspace ID"
// @Param boardId path string true "Board ID"
// @Param columnId path string true "Column ID"
// @Param request body dto.UpdateColumnRequest true "Column payload"
// @Success 200 {object} dto.ColumnDetailResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id}/boards/{boardId}/columns/{columnId} [patch]
func (h *ColumnHandler) Update(c *gin.Context) {
	var req dto.UpdateColumnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	resp, err := h.columns.Update(c.Request.Context(), currentUserID(c), c.Param("id"), c.Param("boardId"), c.Param("columnId"), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Delete deletes a column with its tasks.
// @Summary Delete a column
// @Tags columns
// @Produce json
// @Security BearerAuth
// @Param wsId path string true "Workspace ID"
// @Param boardId path string true "Board ID"
// @Param columnId path string true "Column ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id}/boards/{boardId}/columns/{columnId} [delete]
func (h *ColumnHandler) Delete(c *gin.Context) {
	if err := h.columns.Delete(c.Request.Context(), currentUserID(c), c.Param("id"), c.Param("boardId"), c.Param("columnId")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "column deleted"})
}
