package workspace

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/http/dto/workspace"
	"github.com/tandem/tandem/internal/http/handler/common"
	pworkspace "github.com/tandem/tandem/internal/usecase/workspace"
)

type WorkspaceHandler struct {
	workspaces pworkspace.UseCase
}

func NewWorkspaceHandler(uc pworkspace.UseCase) *WorkspaceHandler {
	return &WorkspaceHandler{workspaces: uc}
}

// @Summary Create a workspace
// @Tags workspaces
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body workspace.CreateWorkspaceRequest true "Workspace payload"
// @Success 201 {object} workspace.WorkspaceResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/workspaces [post]
func (h *WorkspaceHandler) Create(c *gin.Context) {
	var req workspace.CreateWorkspaceRequest
	if !common.ParseJSON(c, &req) {
		return
	}
	resp, err := h.workspaces.Create(c.Request.Context(), common.CurrentUserID(c), req)
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// @Summary List workspaces
// @Tags workspaces
// @Produce json
// @Security BearerAuth
// @Success 200 {array} workspace.WorkspaceResponse
// @Failure 401 {object} map[string]string
// @Router /api/v1/workspaces [get]
func (h *WorkspaceHandler) List(c *gin.Context) {
	resp, err := h.workspaces.List(c.Request.Context(), common.CurrentUserID(c))
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Get a workspace
// @Tags workspaces
// @Produce json
// @Security BearerAuth
// @Param id path string true "Workspace ID"
// @Success 200 {object} workspace.WorkspaceDetailResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id} [get]
func (h *WorkspaceHandler) Get(c *gin.Context) {
	resp, err := h.workspaces.Get(c.Request.Context(), common.CurrentUserID(c), c.Param("id"))
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Update a workspace
// @Tags workspaces
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Workspace ID"
// @Param request body workspace.UpdateWorkspaceRequest true "Workspace payload"
// @Success 200 {object} workspace.WorkspaceResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id} [patch]
func (h *WorkspaceHandler) Update(c *gin.Context) {
	var req workspace.UpdateWorkspaceRequest
	if !common.ParseJSON(c, &req) {
		return
	}
	resp, err := h.workspaces.Update(c.Request.Context(), common.CurrentUserID(c), c.Param("id"), req)
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Delete a workspace
// @Tags workspaces
// @Produce json
// @Security BearerAuth
// @Param id path string true "Workspace ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id} [delete]
func (h *WorkspaceHandler) Delete(c *gin.Context) {
	if err := h.workspaces.Delete(c.Request.Context(), common.CurrentUserID(c), c.Param("id")); err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "workspace deleted"})
}

// @Summary Set workspace theme
// @Tags workspaces
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Workspace ID"
// @Param request body workspace.SetThemeRequest true "Theme payload"
// @Success 200 {object} workspace.WorkspaceResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id}/theme [put]
func (h *WorkspaceHandler) SetTheme(c *gin.Context) {
	var req workspace.SetThemeRequest
	if !common.ParseJSON(c, &req) {
		return
	}
	resp, err := h.workspaces.SetTheme(c.Request.Context(), common.CurrentUserID(c), c.Param("id"), req.Theme)
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Add a workspace member
// @Tags workspaces
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Workspace ID"
// @Param request body workspace.AddMemberRequest true "Member payload"
// @Success 201 {object} workspace.WorkspaceMemberResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/v1/workspaces/{id}/members [post]
func (h *WorkspaceHandler) AddMember(c *gin.Context) {
	var req workspace.AddMemberRequest
	if !common.ParseJSON(c, &req) {
		return
	}
	resp, err := h.workspaces.AddMember(c.Request.Context(), common.CurrentUserID(c), c.Param("id"), req)
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// @Summary Get a workspace invite token
// @Tags workspaces
// @Produce json
// @Security BearerAuth
// @Param id path string true "Workspace ID"
// @Success 200 {object} workspace.WorkspaceInviteResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id}/invite [get]
func (h *WorkspaceHandler) GetInvite(c *gin.Context) {
	resp, err := h.workspaces.GetInvite(c.Request.Context(), common.CurrentUserID(c), c.Param("id"))
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Disable a workspace invite
// @Tags workspaces
// @Produce json
// @Security BearerAuth
// @Param id path string true "Workspace ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id}/invite [delete]
func (h *WorkspaceHandler) DisableInvite(c *gin.Context) {
	if err := h.workspaces.DisableInvite(c.Request.Context(), common.CurrentUserID(c), c.Param("id")); err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "invite disabled"})
}

// @Summary Join a workspace by invite
// @Tags workspaces
// @Produce json
// @Security BearerAuth
// @Param token path string true "Invite token"
// @Success 200 {object} workspace.WorkspaceResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/invite/{token}/join [post]
func (h *WorkspaceHandler) JoinByInvite(c *gin.Context) {
	resp, err := h.workspaces.JoinByInvite(c.Request.Context(), common.CurrentUserID(c), c.Param("token"))
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Change a member's role
// @Tags workspaces
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Workspace ID"
// @Param userId path string true "User ID"
// @Param request body workspace.UpdateMemberRoleRequest true "Role payload"
// @Success 200 {object} workspace.WorkspaceMemberResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id}/members/{userId} [patch]
func (h *WorkspaceHandler) UpdateRole(c *gin.Context) {
	var req workspace.UpdateMemberRoleRequest
	if !common.ParseJSON(c, &req) {
		return
	}
	resp, err := h.workspaces.UpdateRole(c.Request.Context(), common.CurrentUserID(c), c.Param("id"), c.Param("userId"), req)
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Remove a workspace member
// @Tags workspaces
// @Produce json
// @Security BearerAuth
// @Param id path string true "Workspace ID"
// @Param userId path string true "User ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id}/members/{userId} [delete]
func (h *WorkspaceHandler) RemoveMember(c *gin.Context) {
	if err := h.workspaces.RemoveMember(c.Request.Context(), common.CurrentUserID(c), c.Param("id"), c.Param("userId")); err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "member removed"})
}

// @Summary Transfer workspace ownership
// @Tags workspaces
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Workspace ID"
// @Param request body workspace.TransferOwnerRequest true "New owner payload"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id}/owner [post]
func (h *WorkspaceHandler) TransferOwner(c *gin.Context) {
	var req workspace.TransferOwnerRequest
	if !common.ParseJSON(c, &req) {
		return
	}
	if err := h.workspaces.TransferOwner(c.Request.Context(), common.CurrentUserID(c), c.Param("id"), req); err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ownership transferred"})
}
