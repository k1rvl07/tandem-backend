package taskcontracts

import "time"

type CreateTaskRequest struct {
	ColumnID    string
	Title       string
	Description string
	AssigneeID  string
	CuratorID   string
	ParentID    string
	DueDate     string
	IsUrgent    bool
	IsHidden    bool
	ImageKey    string
}

type UpdateTaskRequest struct {
	BoardID     *string
	ColumnID    *string
	Title       *string
	Description *string
	AssigneeID  *string
	CuratorID   *string
	ParentID    *string
	DueDate     *string
	Position    *int
	IsUrgent    *bool
	IsHidden    *bool
	ImageKey    *string
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
	Q               string
	BoardID         string
	AssigneeID      string
	Status          string
	Only            string
	ExcludeSubtasks bool
	Limit           int
	Offset          int
}
