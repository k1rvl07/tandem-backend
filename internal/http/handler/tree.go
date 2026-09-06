package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/http/dto"
	"github.com/tandem/tandem/internal/usecase/tree"
)

type TreeHandler struct {
	tree tree.UseCase
}

func NewTreeHandler(uc tree.UseCase) *TreeHandler {
	return &TreeHandler{tree: uc}
}

// @Summary Tasks tree
// @Tags tasks
// @Produce json
// @Security BearerAuth
// @Param tasks query string false "all|mine|for_me"
// @Param boards query string false "all|fav"
// @Param workspaces query string false "all|fav"
// @Success 200 {array} dto.TreeWorkspaceResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/tasks/tree [get]
func (h *TreeHandler) List(c *gin.Context) {
	var query dto.TreeQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query"})
		return
	}
	resp, err := h.tree.List(c.Request.Context(), currentUserID(c), query)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
