package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/http/dto"
	"github.com/tandem/tandem/internal/usecase/workspace"
)

type WorkspaceHandler struct {
	workspaces workspace.UseCase
}

func NewWorkspaceHandler(uc workspace.UseCase) *WorkspaceHandler {
	return &WorkspaceHandler{workspaces: uc}
}

// Create creates a workspace and adds the caller as its owner.
// @Summary Create a workspace
// @Tags workspaces
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateWorkspaceRequest true "Workspace payload"
// @Success 201 {object} dto.WorkspaceResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/workspaces [post]
func (h *WorkspaceHandler) Create(c *gin.Context) {
	var req dto.CreateWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	resp, err := h.workspaces.Create(c.Request.Context(), currentUserID(c), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// List lists workspaces the caller is a member of.
// @Summary List workspaces
// @Tags workspaces
// @Produce json
// @Security BearerAuth
// @Success 200 {array} dto.WorkspaceResponse
// @Failure 401 {object} map[string]string
// @Router /api/v1/workspaces [get]
func (h *WorkspaceHandler) List(c *gin.Context) {
	resp, err := h.workspaces.List(c.Request.Context(), currentUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Get returns workspace details with its members.
// @Summary Get a workspace
// @Tags workspaces
// @Produce json
// @Security BearerAuth
// @Param id path string true "Workspace ID"
// @Success 200 {object} dto.WorkspaceDetailResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id} [get]
func (h *WorkspaceHandler) Get(c *gin.Context) {
	resp, err := h.workspaces.Get(c.Request.Context(), currentUserID(c), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Update updates workspace name and description.
// @Summary Update a workspace
// @Tags workspaces
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Workspace ID"
// @Param request body dto.UpdateWorkspaceRequest true "Workspace payload"
// @Success 200 {object} dto.WorkspaceResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id} [patch]
func (h *WorkspaceHandler) Update(c *gin.Context) {
	var req dto.UpdateWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	resp, err := h.workspaces.Update(c.Request.Context(), currentUserID(c), c.Param("id"), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Delete deletes a workspace and removes all its members.
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
	if err := h.workspaces.Delete(c.Request.Context(), currentUserID(c), c.Param("id")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "workspace deleted"})
}

// AddMember adds a user to a workspace by login.
// @Summary Add a workspace member
// @Tags workspaces
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Workspace ID"
// @Param request body dto.AddMemberRequest true "Member payload"
// @Success 201 {object} dto.WorkspaceMemberResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/v1/workspaces/{id}/members [post]
func (h *WorkspaceHandler) AddMember(c *gin.Context) {
	var req dto.AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	resp, err := h.workspaces.AddMember(c.Request.Context(), currentUserID(c), c.Param("id"), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// RemoveMember removes a member from a workspace.
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
	if err := h.workspaces.RemoveMember(c.Request.Context(), currentUserID(c), c.Param("id"), c.Param("userId")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "member removed"})
}

// TransferOwner transfers workspace ownership to another member.
// @Summary Transfer workspace ownership
// @Tags workspaces
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Workspace ID"
// @Param request body dto.TransferOwnerRequest true "New owner payload"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id}/owner [post]
func (h *WorkspaceHandler) TransferOwner(c *gin.Context) {
	var req dto.TransferOwnerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := h.workspaces.TransferOwner(c.Request.Context(), currentUserID(c), c.Param("id"), req); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "ownership transferred"})
}
