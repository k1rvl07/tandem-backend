package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/http/dto"
	"github.com/tandem/tandem/internal/usecase/task"
)

type TaskHandler struct {
	tasks task.UseCase
}

func NewTaskHandler(uc task.UseCase) *TaskHandler {
	return &TaskHandler{tasks: uc}
}

// Create adds a task to a column.
// @Summary Create a task
// @Tags tasks
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param wsId path string true "Workspace ID"
// @Param boardId path string true "Board ID"
// @Param request body dto.CreateTaskRequest true "Task payload"
// @Success 201 {object} dto.TaskResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id}/boards/{boardId}/tasks [post]
func (h *TaskHandler) Create(c *gin.Context) {
	var req dto.CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	resp, err := h.tasks.Create(c.Request.Context(), currentUserID(c), c.Param("id"), c.Param("boardId"), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// Update edits a task or moves it between columns.
// @Summary Update a task
// @Tags tasks
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param wsId path string true "Workspace ID"
// @Param boardId path string true "Board ID"
// @Param taskId path string true "Task ID"
// @Param request body dto.UpdateTaskRequest true "Task payload"
// @Success 200 {object} dto.TaskResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id}/boards/{boardId}/tasks/{taskId} [patch]
func (h *TaskHandler) Update(c *gin.Context) {
	var req dto.UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	resp, err := h.tasks.Update(c.Request.Context(), currentUserID(c), c.Param("id"), c.Param("boardId"), c.Param("taskId"), req)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Delete removes a task.
// @Summary Delete a task
// @Tags tasks
// @Produce json
// @Security BearerAuth
// @Param wsId path string true "Workspace ID"
// @Param boardId path string true "Board ID"
// @Param taskId path string true "Task ID"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id}/boards/{boardId}/tasks/{taskId} [delete]
func (h *TaskHandler) Delete(c *gin.Context) {
	if err := h.tasks.Delete(c.Request.Context(), currentUserID(c), c.Param("id"), c.Param("boardId"), c.Param("taskId")); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "task deleted"})
}
