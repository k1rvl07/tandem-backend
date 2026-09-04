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

func (r *UserRepo) FindByLogin(ctx context.Context, login string) (*models.User, error) {
	var e entity.User
	err := r.db.WithContext(ctx).Where("login = ?", login).First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, err)
	}
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return toDomain(&e), nil
}

func (r *UserRepo) ExistsByLogin(ctx context.Context, login string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&entity.User{}).Where("login = ?", login).Count(&count).Error
	if err != nil {
		return false, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return count > 0, nil
}

func (r *UserRepo) List(ctx context.Context) ([]*models.User, error) {
	var es []entity.User
	err := r.db.WithContext(ctx).Order("created_at ASC").Find(&es).Error
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	users := make([]*models.User, 0, len(es))
	for i := range es {
		users = append(users, toDomain(&es[i]))
	}
	return users, nil
}

func (r *UserRepo) Update(ctx context.Context, user *models.User) error {
	e := toEntity(user)
	err := r.db.WithContext(ctx).Model(&entity.User{}).Where("id = ?", e.ID).Updates(map[string]interface{}{
		"login":         e.Login,
		"password_hash": e.PasswordHash,
		"role":          e.Role,
		"display_name":  e.DisplayName,
		"bio":           e.Bio,
		"avatar_key":    e.AvatarKey,
	}).Error
	if err != nil {
		if isUniqueViolation(err) {
			return pkgerrors.Wrap(pkgerrors.ErrConflict, err)
		}
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return nil
}

func (r *UserRepo) Delete(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&entity.User{})
	if res.Error != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	return nil
}

func toEntity(u *models.User) *entity.User {
	return &entity.User{
		ID:           u.ID,
		Login:        u.Login,
		PasswordHash: u.PasswordHash,
		Role:         u.Role,
		DisplayName:  u.DisplayName,
		Bio:          u.Bio,
		AvatarKey:    u.AvatarKey,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}

func toDomain(e *entity.User) *models.User {
	return &models.User{
		ID:           e.ID,
		Login:        e.Login,
		PasswordHash: e.PasswordHash,
		Role:         e.Role,
		DisplayName:  e.DisplayName,
		Bio:          e.Bio,
		AvatarKey:    e.AvatarKey,
		CreatedAt:    e.CreatedAt,
		UpdatedAt:    e.UpdatedAt,
	}
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique")
}

var _ repository.UserRepository = (*UserRepo)(nil)
