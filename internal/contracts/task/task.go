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
	ID          string
	Login       string
	DisplayName string
	AvatarKey   string
}

type TaskResponse struct {
	ID          string
	DisplayID   string
	WorkspaceID string
	BoardID     string
	BoardName   string
	ColumnID    string
	ColumnName  string
	Title       string
	Description string
	Author      *TaskUserResponse
	Assignee    *TaskUserResponse
	Curator     *TaskUserResponse
	ParentID    string
	DueDate     *time.Time
	Position    int
	IsUrgent    bool
	IsHidden    bool
	ImageKey    string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type TaskReference struct {
	ID          string
	DisplayID   string
	Title       string
	WorkspaceID string
	BoardID     string
	BoardName   string
	ColumnID    string
	ColumnName  string
	IsUrgent    bool
	IsHidden    bool
}

type TaskDetailResponse struct {
	TaskResponse
	Parent   *TaskReference
	Subtasks []TaskReference
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
