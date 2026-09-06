package repository

import (
	"context"
	"errors"

	"github.com/tandem/tandem/internal/domain/models"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/repository/entity"
	"gorm.io/gorm"
)

type AttachmentRepo struct {
	db *gorm.DB
}

func NewAttachmentRepo(db *gorm.DB) *AttachmentRepo {
	return &AttachmentRepo{db: db}
}

func (r *AttachmentRepo) CreateAttachment(ctx context.Context, attachment *models.TaskAttachment) error {
	e := &entity.TaskAttachment{
		ID:          attachment.ID,
		TaskID:      attachment.TaskID,
		Filename:    attachment.Filename,
		ObjectKey:   attachment.ObjectKey,
		Size:        attachment.Size,
		ContentType: attachment.ContentType,
		UploadedBy:  attachment.UploadedBy,
	}
	err := r.db.WithContext(ctx).Create(e).Error
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	attachment.CreatedAt = e.CreatedAt
	return nil
}

func (r *AttachmentRepo) FindAttachmentByID(ctx context.Context, id string) (*models.TaskAttachment, error) {
	var e entity.TaskAttachment
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, err)
	}
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return &models.TaskAttachment{
		ID:          e.ID,
		TaskID:      e.TaskID,
		Filename:    e.Filename,
		ObjectKey:   e.ObjectKey,
		Size:        e.Size,
		ContentType: e.ContentType,
		UploadedBy:  e.UploadedBy,
		CreatedAt:   e.CreatedAt,
	}, nil
}

func (r *AttachmentRepo) ListAttachmentsByTask(ctx context.Context, taskID string) ([]models.TaskAttachment, error) {
	var es []entity.TaskAttachment
	err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("created_at ASC").Find(&es).Error
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	attachments := make([]models.TaskAttachment, 0, len(es))
	for i := range es {
		attachments = append(attachments, models.TaskAttachment{
			ID:          es[i].ID,
			TaskID:      es[i].TaskID,
			Filename:    es[i].Filename,
			ObjectKey:   es[i].ObjectKey,
			Size:        es[i].Size,
			ContentType: es[i].ContentType,
			UploadedBy:  es[i].UploadedBy,
			CreatedAt:   es[i].CreatedAt,
		})
	}
	return attachments, nil
}

func (r *AttachmentRepo) DeleteAttachment(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&entity.TaskAttachment{})
	if res.Error != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	return nil
}

var _ repository.AttachmentRepository = (*AttachmentRepo)(nil)

type FavoriteRepo struct {
	db *gorm.DB
}

func NewFavoriteRepo(db *gorm.DB) *FavoriteRepo {
	return &FavoriteRepo{db: db}
}

func (r *FavoriteRepo) AddFavorite(ctx context.Context, favorite *models.Favorite) error {
	e := &entity.Favorite{
		ID:         favorite.ID,
		UserID:     favorite.UserID,
		TargetType: favorite.TargetType,
		TargetID:   favorite.TargetID,
	}
	err := r.db.WithContext(ctx).Create(e).Error
	if err != nil {
		if isUniqueViolation(err) {
			return nil
		}
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	favorite.CreatedAt = e.CreatedAt
	return nil
}

func (r *FavoriteRepo) RemoveFavorite(ctx context.Context, userID, targetType, targetID string) error {
	res := r.db.WithContext(ctx).
		Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).
		Delete(&entity.Favorite{})
	if res.Error != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, res.Error)
	}
	return nil
}

func (r *FavoriteRepo) IsFavorite(ctx context.Context, userID, targetType, targetID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&entity.Favorite{}).
		Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).
		Count(&count).Error
	if err != nil {
		return false, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return count > 0, nil
}

func (r *FavoriteRepo) ListFavoriteTargets(ctx context.Context, userID, targetType string) (map[string]bool, error) {
	var es []entity.Favorite
	err := r.db.WithContext(ctx).Where("user_id = ? AND target_type = ?", userID, targetType).Find(&es).Error
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	result := make(map[string]bool, len(es))
	for i := range es {
		result[es[i].TargetID] = true
	}
	return result, nil
}

var _ repository.FavoriteRepository = (*FavoriteRepo)(nil)
