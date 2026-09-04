package dto

import "time"

type CreateBoardRequest struct {
	Name string `json:"name"`
}

type UpdateBoardRequest struct {
	Name string `json:"name"`
}

type BoardResponse struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Name        string    `json:"name"`
	Position    int       `json:"position"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TaskAssigneeResponse struct {
	ID          string `json:"id"`
	Login       string `json:"login"`
	DisplayName string `json:"display_name"`
	AvatarKey   string `json:"avatar_key"`
}

type TaskResponse struct {
	ID          string                `json:"id"`
	ColumnID    string                `json:"column_id"`
	Title       string                `json:"title"`
	Description string                `json:"description"`
	Priority    string                `json:"priority"`
	Assignee    *TaskAssigneeResponse `json:"assignee"`
	DueDate     *time.Time            `json:"due_date"`
	Position    int                   `json:"position"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
}

type ColumnResponse struct {
	ID        string    `json:"id"`
	BoardID   string    `json:"board_id"`
	Name      string    `json:"name"`
	Position  int       `json:"position"`
	TaskCount int       `json:"task_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ColumnDetailResponse struct {
	ID        string         `json:"id"`
	BoardID   string         `json:"board_id"`
	Name      string         `json:"name"`
	Position  int            `json:"position"`
	TaskCount int            `json:"task_count"`
	Tasks     []TaskResponse `json:"tasks"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type BoardDetailResponse struct {
	ID          string                 `json:"id"`
	WorkspaceID string                 `json:"workspace_id"`
	Name        string                 `json:"name"`
	Position    int                    `json:"position"`
	Columns     []ColumnDetailResponse `json:"columns"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}
