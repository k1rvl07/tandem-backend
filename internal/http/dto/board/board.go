package board

import (
	"time"

	"github.com/tandem/tandem/internal/http/dto/task"
)

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
	IsMain      bool      `json:"is_main"`
	IsFavorite  bool      `json:"is_favorite"`
	TaskCount   int       `json:"task_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ReorderBoardsRequest struct {
	BoardIDs []string `json:"board_ids"`
}

type ColumnDetailResponse struct {
	ID        string              `json:"id"`
	BoardID   string              `json:"board_id"`
	Name      string              `json:"name"`
	Position  int                 `json:"position"`
	TaskCount int                 `json:"task_count"`
	Tasks     []task.TaskResponse `json:"tasks"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
}

type BoardDetailResponse struct {
	ID          string                 `json:"id"`
	WorkspaceID string                 `json:"workspace_id"`
	Name        string                 `json:"name"`
	Position    int                    `json:"position"`
	IsMain      bool                   `json:"is_main"`
	Columns     []ColumnDetailResponse `json:"columns"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}
