package task

import "time"

type CreateTaskRequest struct {
	ColumnID    string `json:"column_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	AssigneeID  string `json:"assignee_id"`
	CuratorID   string `json:"curator_id"`
	ParentID    string `json:"parent_id"`
	DueDate     string `json:"due_date"`
	IsUrgent    bool   `json:"is_urgent"`
	IsHidden    bool   `json:"is_hidden"`
	ImageKey    string `json:"image_key"`
}

type UpdateTaskRequest struct {
	BoardID     *string `json:"board_id"`
	ColumnID    *string `json:"column_id"`
	Title       *string `json:"title"`
	Description *string `json:"description"`
	AssigneeID  *string `json:"assignee_id"`
	CuratorID   *string `json:"curator_id"`
	ParentID    *string `json:"parent_id"`
	DueDate     *string `json:"due_date"`
	Position    *int    `json:"position"`
	IsUrgent    *bool   `json:"is_urgent"`
	IsHidden    *bool   `json:"is_hidden"`
	ImageKey    *string `json:"image_key"`
}

type TaskUserResponse struct {
	ID          string `json:"id"`
	Login       string `json:"login"`
	DisplayName string `json:"display_name"`
	AvatarKey   string `json:"avatar_key"`
}

type TaskResponse struct {
	ID          string            `json:"id"`
	DisplayID   string            `json:"display_id"`
	WorkspaceID string            `json:"workspace_id"`
	BoardID     string            `json:"board_id"`
	BoardName   string            `json:"board_name"`
	ColumnID    string            `json:"column_id"`
	ColumnName  string            `json:"column_name"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Author      *TaskUserResponse `json:"author"`
	Assignee    *TaskUserResponse `json:"assignee"`
	Curator     *TaskUserResponse `json:"curator"`
	ParentID    string            `json:"parent_id"`
	DueDate     *time.Time        `json:"due_date"`
	Position    int               `json:"position"`
	IsUrgent    bool              `json:"is_urgent"`
	IsHidden    bool              `json:"is_hidden"`
	ImageKey    string            `json:"image_key"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type TaskReference struct {
	ID          string `json:"id"`
	DisplayID   string `json:"display_id"`
	Title       string `json:"title"`
	WorkspaceID string `json:"workspace_id"`
	BoardID     string `json:"board_id"`
	BoardName   string `json:"board_name"`
	ColumnID    string `json:"column_id"`
	ColumnName  string `json:"column_name"`
	IsUrgent    bool   `json:"is_urgent"`
	IsHidden    bool   `json:"is_hidden"`
}

type TaskDetailResponse struct {
	TaskResponse
	Parent   *TaskReference  `json:"parent"`
	Subtasks []TaskReference `json:"subtasks"`
}

type ListWorkspaceTasksQuery struct {
	Q               string `form:"q"`
	BoardID         string `form:"board_id"`
	AssigneeID      string `form:"assignee_id"`
	Status          string `form:"status"`
	Only            string `form:"only"`
	ExcludeSubtasks bool   `form:"exclude_subtasks"`
	Limit           int    `form:"limit"`
	Offset          int    `form:"offset"`
}
