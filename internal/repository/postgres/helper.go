package postgres

import (
	"context"
	"errors"

	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"gorm.io/gorm"
)

type Helper struct {
	DB *gorm.DB
}

func NewHelper(db *gorm.DB) *Helper {
	return &Helper{DB: db}
}

func (h *Helper) First(ctx context.Context, out interface{}, query string, args ...interface{}) error {
	db := h.DB.WithContext(ctx)
	if query != "" {
		db = db.Where(query, args...)
	}
	err := db.First(out).Error
	return wrapRepoErr(err)
}

func (h *Helper) Find(ctx context.Context, out interface{}, query string, args ...interface{}) error {
	db := h.DB.WithContext(ctx)
	if query != "" {
		db = db.Where(query, args...)
	}
	err := db.Find(out).Error
	return wrapRepoErr(err)
}

func (h *Helper) List(ctx context.Context, out interface{}, order, query string, args ...interface{}) error {
	db := h.DB.WithContext(ctx).Order(order)
	if query != "" {
		db = db.Where(query, args...)
	}
	err := db.Find(out).Error
	return wrapRepoErr(err)
}

func (h *Helper) Exists(ctx context.Context, model interface{}, query string, args ...interface{}) (bool, error) {
	var one int
	db := h.DB.WithContext(ctx).Model(model).Select("1")
	if query != "" {
		db = db.Where(query, args...)
	}
	err := db.Limit(1).Scan(&one).Error
	if err != nil {
		return false, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return one == 1, nil
}

func (h *Helper) Paginate(ctx context.Context, out interface{}, query string, args []interface{}, order string, limit, offset int) error {
	db := h.DB.WithContext(ctx).Order(order).Limit(limit).Offset(offset)
	if query != "" {
		db = db.Where(query, args...)
	}
	err := db.Find(out).Error
	return wrapRepoErr(err)
}

func (h *Helper) Count(ctx context.Context, model interface{}, query string, args ...interface{}) (int64, error) {
	db := h.DB.WithContext(ctx).Model(model)
	if query != "" {
		db = db.Where(query, args...)
	}
	var count int64
	err := db.Count(&count).Error
	if err != nil {
		return 0, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return count, nil
}

func wrapRepoErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, err)
	}
	return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
}
