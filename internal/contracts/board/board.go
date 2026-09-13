package boardcontracts

import (
	"time"

	taskcontracts "github.com/tandem/tandem/internal/contracts/task"
)

type CreateBoardRequest struct {
	Name string
}

type UpdateBoardRequest struct {
	Name string
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
	BoardIDs []string
}

type ColumnDetailResponse struct {
	ID        string
	BoardID   string
	Name      string
	Position  int
	TaskCount int
	Tasks     []taskcontracts.TaskResponse
	CreatedAt time.Time
	UpdatedAt time.Time
}

type BoardDetailResponse struct {
	ID          string
	WorkspaceID string
	Name        string
	Position    int
	IsMain      bool
	Columns     []ColumnDetailResponse
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
