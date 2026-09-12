package board

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/http/dto/board"
	"github.com/tandem/tandem/internal/http/handler/common"
	pboard "github.com/tandem/tandem/internal/usecase/board"
)

type BoardHandler struct {
	boards pboard.UseCase
}

func NewBoardHandler(uc pboard.UseCase) *BoardHandler {
	return &BoardHandler{boards: uc}
}

// @Summary List workspace boards
// @Tags boards
// @Produce json
// @Security BearerAuth
// @Param wsId path string true "Workspace ID"
// @Success 200 {array} board.BoardResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /api/v1/workspaces/{id}/boards [get]
func (h *BoardHandler) List(c *gin.Context) {
	resp, err := h.boards.List(c.Request.Context(), common.CurrentUserID(c), c.Param("id"))
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Create a board
// @Tags boards
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param wsId path string true "Workspace ID"
// @Param request body board.CreateBoardRequest true "Board payload"
// @Success 201 {object} board.BoardResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /api/v1/workspaces/{id}/boards [post]
func (h *BoardHandler) Create(c *gin.Context) {
	var req board.CreateBoardRequest
	if !common.ParseJSON(c, &req) {
		return
	}
	resp, err := h.boards.Create(c.Request.Context(), common.CurrentUserID(c), c.Param("id"), req)
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// @Summary Get a board
// @Tags boards
// @Produce json
// @Security BearerAuth
// @Param wsId path string true "Workspace ID"
// @Param boardId path string true "Board ID"
// @Success 200 {object} board.BoardDetailResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id}/boards/{boardId} [get]
func (h *BoardHandler) Get(c *gin.Context) {
	resp, err := h.boards.Get(c.Request.Context(), common.CurrentUserID(c), c.Param("id"), c.Param("boardId"))
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Set main board
// @Tags boards
// @Produce json
// @Security BearerAuth
// @Param wsId path string true "Workspace ID"
// @Param boardId path string true "Board ID"
// @Success 200 {object} board.BoardResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id}/boards/{boardId}/main [put]
func (h *BoardHandler) SetMain(c *gin.Context) {
	resp, err := h.boards.SetMain(c.Request.Context(), common.CurrentUserID(c), c.Param("id"), c.Param("boardId"))
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Reorder boards
// @Tags boards
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param wsId path string true "Workspace ID"
// @Param request body board.ReorderBoardsRequest true "Ordered board ids"
// @Success 200 {array} board.BoardResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /api/v1/workspaces/{id}/boards/reorder [put]
func (h *BoardHandler) Reorder(c *gin.Context) {
	var req board.ReorderBoardsRequest
	if !common.ParseJSON(c, &req) {
		return
	}
	resp, err := h.boards.Reorder(c.Request.Context(), common.CurrentUserID(c), c.Param("id"), req)
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Update a board
// @Tags boards
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param wsId path string true "Workspace ID"
// @Param boardId path string true "Board ID"
// @Param request body board.UpdateBoardRequest true "Board payload"
// @Success 200 {object} board.BoardResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id}/boards/{boardId} [patch]
func (h *BoardHandler) Update(c *gin.Context) {
	var req board.UpdateBoardRequest
	if !common.ParseJSON(c, &req) {
		return
	}
	resp, err := h.boards.Update(c.Request.Context(), common.CurrentUserID(c), c.Param("id"), c.Param("boardId"), req)
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Delete a board
// @Tags boards
// @Produce json
// @Security BearerAuth
// @Param wsId path string true "Workspace ID"
// @Param boardId path string true "Board ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id}/boards/{boardId} [delete]
func (h *BoardHandler) Delete(c *gin.Context) {
	if err := h.boards.Delete(c.Request.Context(), common.CurrentUserID(c), c.Param("id"), c.Param("boardId")); err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "board deleted"})
}
