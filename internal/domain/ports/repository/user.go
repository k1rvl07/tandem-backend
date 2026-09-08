package repository

import (
	"context"

	muser "github.com/tandem/tandem/internal/domain/models/user"
)

type UserRepository interface {
	Create(ctx context.Context, user *muser.User) error
	FindByID(ctx context.Context, id string) (*muser.User, error)
	FindByLogin(ctx context.Context, login string) (*muser.User, error)
	ExistsByLogin(ctx context.Context, login string) (bool, error)
	List(ctx context.Context) ([]*muser.User, error)
	ListPage(ctx context.Context, query string, limit, offset int) ([]*muser.User, error)
	Count(ctx context.Context, query string) (int, error)
	Update(ctx context.Context, user *muser.User) error
	Delete(ctx context.Context, id string) error
}
