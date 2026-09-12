package tree

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/http/dto/tree"
	"github.com/tandem/tandem/internal/http/handler/common"
	ptree "github.com/tandem/tandem/internal/usecase/tree"
)

type TreeHandler struct {
	tree ptree.UseCase
}

func NewTreeHandler(uc ptree.UseCase) *TreeHandler {
	return &TreeHandler{tree: uc}
}

// @Summary Tasks tree
// @Tags tasks
// @Produce json
// @Security BearerAuth
// @Param tasks query string false "all|mine|for_me"
// @Param boards query string false "all|fav"
// @Param workspaces query string false "all|fav"
// @Success 200 {array} tree.TreeWorkspaceResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/tasks/tree [get]
func (h *TreeHandler) List(c *gin.Context) {
	var query tree.TreeQuery
	if !common.ParseQuery(c, &query, "invalid query") {
		return
	}
	resp, err := h.tree.List(c.Request.Context(), common.CurrentUserID(c), query)
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
