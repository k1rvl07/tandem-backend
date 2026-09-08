package repository

import (
	"context"

	mboard "github.com/tandem/tandem/internal/domain/models/board"
)

type BoardRepository interface {
	CreateBoard(ctx context.Context, board *mboard.Board) error
	FindBoardByID(ctx context.Context, id string) (*mboard.Board, error)
	UpdateBoard(ctx context.Context, board *mboard.Board) error
	DeleteBoard(ctx context.Context, id string) error
	ListBoards(ctx context.Context, workspaceID string) ([]*mboard.Board, error)
	ClearMainBoards(ctx context.Context, workspaceID string) error
	CountTasksByBoard(ctx context.Context, workspaceID string) (map[string]int, error)
	ReorderBoards(ctx context.Context, workspaceID string, boardIDs []string) error
}
