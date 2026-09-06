package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/usecase/favorite"
)

type FavoriteHandler struct {
	favorites favorite.UseCase
}

func NewFavoriteHandler(uc favorite.UseCase) *FavoriteHandler {
	return &FavoriteHandler{favorites: uc}
}

// @Summary Favorite a workspace
// @Tags favorites
// @Produce json
// @Security BearerAuth
// @Param workspaceId path string true "Workspace ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/favorites/workspaces/{workspaceId} [put]
func (h *FavoriteHandler) AddWorkspace(c *gin.Context) {
	if err := h.favorites.Add(c.Request.Context(), currentUserID(c), "workspace", c.Param("workspaceId")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "favorited"})
}

// @Summary Unfavorite a workspace
// @Tags favorites
// @Produce json
// @Security BearerAuth
// @Param workspaceId path string true "Workspace ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /api/v1/favorites/workspaces/{workspaceId} [delete]
func (h *FavoriteHandler) RemoveWorkspace(c *gin.Context) {
	if err := h.favorites.Remove(c.Request.Context(), currentUserID(c), "workspace", c.Param("workspaceId")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "unfavorited"})
}

// @Summary Favorite a board
// @Tags favorites
// @Produce json
// @Security BearerAuth
// @Param boardId path string true "Board ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/favorites/boards/{boardId} [put]
func (h *FavoriteHandler) AddBoard(c *gin.Context) {
	if err := h.favorites.Add(c.Request.Context(), currentUserID(c), "board", c.Param("boardId")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "favorited"})
}

// @Summary Unfavorite a board
// @Tags favorites
// @Produce json
// @Security BearerAuth
// @Param boardId path string true "Board ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /api/v1/favorites/boards/{boardId} [delete]
func (h *FavoriteHandler) RemoveBoard(c *gin.Context) {
	if err := h.favorites.Remove(c.Request.Context(), currentUserID(c), "board", c.Param("boardId")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "unfavorited"})
}
