package repository

import (
	"context"

	"github.com/tandem/tandem/internal/domain/models"
)

type BoardRepository interface {
	CreateBoard(ctx context.Context, board *models.Board) error
	FindBoardByID(ctx context.Context, id string) (*models.Board, error)
	UpdateBoard(ctx context.Context, board *models.Board) error
	DeleteBoard(ctx context.Context, id string) error
	ListBoards(ctx context.Context, workspaceID string) ([]*models.Board, error)
}
