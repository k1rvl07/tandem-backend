package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/tandem/tandem/internal/domain/models"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/repository/entity"
	"gorm.io/gorm"
)

type UserRepo struct {
	db *gorm.DB
}

func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, user *models.User) error {
	e := toEntity(user)
	err := r.db.WithContext(ctx).Create(e).Error
	if err != nil {
		if isUniqueViolation(err) {
			return pkgerrors.Wrap(pkgerrors.ErrConflict, err)
		}
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	d := toDomain(e)
	user.CreatedAt = d.CreatedAt
	user.UpdatedAt = d.UpdatedAt
	return nil
}

func (r *UserRepo) FindByID(ctx context.Context, id string) (*models.User, error) {
	var e entity.User
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, err)
	}
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return toDomain(&e), nil
}

func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var e entity.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, err)
	}
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return toDomain(&e), nil
}

func (r *UserRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.User{}).Where("email = ?", email).Count(&count).Error
	if err != nil {
		return false, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return count > 0, nil
}

func toEntity(u *models.User) *entity.User {
	return &entity.User{
		ID:           u.ID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

func toDomain(e *entity.User) *models.User {
	return &models.User{
		ID:           e.ID,
		Email:        e.Email,
		PasswordHash: e.PasswordHash,
		CreatedAt:    e.CreatedAt,
		UpdatedAt:    e.UpdatedAt,
	}
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique")
}

var _ repository.UserRepository = (*UserRepo)(nil)
