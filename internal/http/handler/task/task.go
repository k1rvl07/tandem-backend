package task

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/http/dto/task"
	"github.com/tandem/tandem/internal/http/handler/common"
	ptask "github.com/tandem/tandem/internal/usecase/task"
)

type TaskHandler struct {
	tasks ptask.UseCase
}

func NewTaskHandler(uc ptask.UseCase) *TaskHandler {
	return &TaskHandler{tasks: uc}
}

// @Summary Create a task
// @Tags tasks
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param wsId path string true "Workspace ID"
// @Param boardId path string true "Board ID"
// @Param request body task.CreateTaskRequest true "Task payload"
// @Success 201 {object} task.TaskResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id}/boards/{boardId}/tasks [post]
func (h *TaskHandler) Create(c *gin.Context) {
	var req task.CreateTaskRequest
	if !common.ParseJSON(c, &req) {
		return
	}
	resp, err := h.tasks.Create(c.Request.Context(), common.CurrentUserID(c), c.Param("id"), c.Param("boardId"), req)
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// @Summary Update a task
// @Tags tasks
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param wsId path string true "Workspace ID"
// @Param boardId path string true "Board ID"
// @Param taskId path string true "Task ID"
// @Param request body task.UpdateTaskRequest true "Task payload"
// @Success 200 {object} task.TaskResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id}/boards/{boardId}/tasks/{taskId} [patch]
func (h *TaskHandler) Update(c *gin.Context) {
	var req task.UpdateTaskRequest
	if !common.ParseJSON(c, &req) {
		return
	}
	resp, err := h.tasks.Update(c.Request.Context(), common.CurrentUserID(c), c.Param("id"), c.Param("boardId"), c.Param("taskId"), req)
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Get a task
// @Tags tasks
// @Produce json
// @Security BearerAuth
// @Param wsId path string true "Workspace ID"
// @Param taskId path string true "Task ID"
// @Success 200 {object} task.TaskDetailResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/workspaces/{id}/tasks/{taskId} [get]
func (h *TaskHandler) Get(c *gin.Context) {
	resp, err := h.tasks.Get(c.Request.Context(), common.CurrentUserID(c), c.Param("id"), c.Param("taskId"))
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary List workspace tasks
// @Tags tasks
// @Produce json
// @Security BearerAuth
// @Param wsId path string true "Workspace ID"
// @Param q query string false "Search in title"
// @Param board_id query string false "Filter by board"
// @Param assignee_id query string false "Filter by assignee"
// @Param status query string false "Filter by column name"
// @Param only query string false "mine|for_me"
// @Param exclude_subtasks query bool false "Exclude subtasks"
// @Success 200 {array} task.TaskResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /api/v1/workspaces/{id}/tasks [get]
func (h *TaskHandler) List(c *gin.Context) {
	var query task.ListWorkspaceTasksQuery
	if !common.ParseQuery(c, &query, "invalid query") {
		return
	}
	resp, err := h.tasks.List(c.Request.Context(), common.CurrentUserID(c), c.Param("id"), query)
	if err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

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
	if err := h.tasks.Delete(c.Request.Context(), common.CurrentUserID(c), c.Param("id"), c.Param("boardId"), c.Param("taskId")); err != nil {
		common.RespondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "task deleted"})
}
