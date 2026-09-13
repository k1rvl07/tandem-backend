package user

import (
	"context"
	"strings"

	muser "github.com/tandem/tandem/internal/domain/models/user"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	euser "github.com/tandem/tandem/internal/repository/entity/user"
	"github.com/tandem/tandem/internal/repository/postgres"
	"gorm.io/gorm"
)

type UserRepo struct {
	helper *postgres.Helper
}

func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{helper: postgres.NewHelper(db)}
}

func (r *UserRepo) Create(ctx context.Context, user *muser.User) error {
	e := toEntity(user)
	err := r.helper.DB.WithContext(ctx).Create(e).Error
	if err != nil {
		if postgres.IsUniqueViolation(err) {
			return pkgerrors.Wrap(pkgerrors.ErrConflict, err)
		}
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	d := toDomain(e)
	user.CreatedAt = d.CreatedAt
	user.UpdatedAt = d.UpdatedAt
	return nil
}

func (r *UserRepo) FindByID(ctx context.Context, id string) (*muser.User, error) {
	var e euser.User
	if err := r.helper.First(ctx, &e, "id = ?", id); err != nil {
		return nil, err
	}
	return toDomain(&e), nil
}

func (r *UserRepo) FindByLogin(ctx context.Context, login string) (*muser.User, error) {
	var e euser.User
	if err := r.helper.First(ctx, &e, "login = ?", login); err != nil {
		return nil, err
	}
	return toDomain(&e), nil
}

func (r *UserRepo) ExistsByLogin(ctx context.Context, login string) (bool, error) {
	return r.helper.Exists(ctx, &euser.User{}, "login = ?", login)
}

func (r *UserRepo) List(ctx context.Context) ([]*muser.User, error) {
	var es []euser.User
	if err := r.helper.DB.WithContext(ctx).Order("created_at ASC").Find(&es).Error; err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	users := make([]*muser.User, 0, len(es))
	for i := range es {
		users = append(users, toDomain(&es[i]))
	}
	return users, nil
}

func likePattern(q string) string {
	q = strings.ReplaceAll(q, `\`, `\\`)
	q = strings.ReplaceAll(q, `%`, `\%`)
	q = strings.ReplaceAll(q, `_`, `\_`)
	return "%" + q + "%"
}

func (r *UserRepo) ListPage(ctx context.Context, query string, limit, offset int) ([]*muser.User, error) {
	var es []euser.User
	args := []interface{}{}
	where := ""
	if query != "" {
		where = "login ILIKE ? ESCAPE '\\'"
		args = append(args, likePattern(query))
	}
	if err := r.helper.Paginate(ctx, &es, where, args, "created_at ASC", limit, offset); err != nil {
		return nil, err
	}
	users := make([]*muser.User, 0, len(es))
	for i := range es {
		users = append(users, toDomain(&es[i]))
	}
	return users, nil
}

func (r *UserRepo) Count(ctx context.Context, query string) (int, error) {
	where := ""
	args := []interface{}{}
	if query != "" {
		where = "login ILIKE ? ESCAPE '\\'"
		args = append(args, likePattern(query))
	}
	total, err := r.helper.Count(ctx, &euser.User{}, where, args...)
	if err != nil {
		return 0, err
	}
	return int(total), nil
}

func (r *UserRepo) Update(ctx context.Context, user *muser.User) error {
	e := toEntity(user)
	err := r.helper.DB.WithContext(ctx).Model(&euser.User{}).Where("id = ?", e.ID).Updates(map[string]interface{}{
		"login":         e.Login,
		"password_hash": e.PasswordHash,
		"role":          e.Role,
		"display_name":  e.DisplayName,
		"bio":           e.Bio,
		"avatar_key":    e.AvatarKey,
	}).Error
	if err != nil {
		if postgres.IsUniqueViolation(err) {
			return pkgerrors.Wrap(pkgerrors.ErrConflict, err)
		}
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return nil
}

func (r *UserRepo) Delete(ctx context.Context, id string) error {
	res := r.helper.DB.WithContext(ctx).Where("id = ?", id).Delete(&euser.User{})
	if res.Error != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	return nil
}

func toEntity(u *muser.User) *euser.User {
	return &euser.User{
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

func toDomain(e *euser.User) *muser.User {
	return &muser.User{
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

var _ repository.UserRepository = (*UserRepo)(nil)
