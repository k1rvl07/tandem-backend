package repository

import (
	"context"

	"github.com/tandem/tandem/internal/domain/models"
)

type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	FindByID(ctx context.Context, id string) (*models.User, error)
	FindByLogin(ctx context.Context, login string) (*models.User, error)
	ExistsByLogin(ctx context.Context, login string) (bool, error)
	List(ctx context.Context) ([]*models.User, error)
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, id string) error
}
