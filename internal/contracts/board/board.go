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
	ID          string
	WorkspaceID string
	Name        string
	Position    int
	IsMain      bool
	IsFavorite  bool
	TaskCount   int
	CreatedAt   time.Time
	UpdatedAt   time.Time
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
