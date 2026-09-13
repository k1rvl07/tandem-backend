package taskext

import (
	"context"

	mattachment "github.com/tandem/tandem/internal/domain/models/attachment"
	mfavorite "github.com/tandem/tandem/internal/domain/models/favorite"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	efavorite "github.com/tandem/tandem/internal/repository/entity/favorite"
	"github.com/tandem/tandem/internal/repository/entity/nullstr"
	etask "github.com/tandem/tandem/internal/repository/entity/task"
	"github.com/tandem/tandem/internal/repository/postgres"
	"gorm.io/gorm"
)

type AttachmentRepo struct {
	helper *postgres.Helper
}

func NewAttachmentRepo(db *gorm.DB) *AttachmentRepo {
	return &AttachmentRepo{helper: postgres.NewHelper(db)}
}

func (r *AttachmentRepo) CreateAttachment(ctx context.Context, attachment *mattachment.TaskAttachment) error {
	e := &etask.TaskAttachment{
		ID:          attachment.ID,
		TaskID:      attachment.TaskID,
		Filename:    attachment.Filename,
		ObjectKey:   attachment.ObjectKey,
		Size:        attachment.Size,
		ContentType: attachment.ContentType,
		UploadedBy:  nullstr.NilIfEmpty(attachment.UploadedBy),
	}
	err := r.helper.DB.WithContext(ctx).Create(e).Error
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	attachment.CreatedAt = e.CreatedAt
	return nil
}

func (r *AttachmentRepo) FindAttachmentByID(ctx context.Context, id string) (*mattachment.TaskAttachment, error) {
	var e etask.TaskAttachment
	if err := r.helper.First(ctx, &e, "id = ?", id); err != nil {
		return nil, err
	}
	return &mattachment.TaskAttachment{
		ID:          e.ID,
		TaskID:      e.TaskID,
		Filename:    e.Filename,
		ObjectKey:   e.ObjectKey,
		Size:        e.Size,
		ContentType: e.ContentType,
		UploadedBy:  nullstr.OrEmpty(e.UploadedBy),
		CreatedAt:   e.CreatedAt,
	}, nil
}

func (r *AttachmentRepo) ListAttachmentsByTask(ctx context.Context, taskID string) ([]mattachment.TaskAttachment, error) {
	var es []etask.TaskAttachment
	if err := r.helper.List(ctx, &es, "created_at ASC", "task_id = ?", taskID); err != nil {
		return nil, err
	}
	attachments := make([]mattachment.TaskAttachment, 0, len(es))
	for i := range es {
		attachments = append(attachments, mattachment.TaskAttachment{
			ID:          es[i].ID,
			TaskID:      es[i].TaskID,
			Filename:    es[i].Filename,
			ObjectKey:   es[i].ObjectKey,
			Size:        es[i].Size,
			ContentType: es[i].ContentType,
			UploadedBy:  nullstr.OrEmpty(es[i].UploadedBy),
			CreatedAt:   es[i].CreatedAt,
		})
	}
	return attachments, nil
}

func (r *AttachmentRepo) DeleteAttachment(ctx context.Context, id string) error {
	res := r.helper.DB.WithContext(ctx).Where("id = ?", id).Delete(&etask.TaskAttachment{})
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
	helper *postgres.Helper
}

func NewFavoriteRepo(db *gorm.DB) *FavoriteRepo {
	return &FavoriteRepo{helper: postgres.NewHelper(db)}
}

func (r *FavoriteRepo) AddFavorite(ctx context.Context, favorite *mfavorite.Favorite) error {
	e := &efavorite.Favorite{
		ID:         favorite.ID,
		UserID:     favorite.UserID,
		TargetType: favorite.TargetType,
		TargetID:   favorite.TargetID,
	}
	err := r.helper.DB.WithContext(ctx).Create(e).Error
	if err != nil {
		if postgres.IsUniqueViolation(err) {
			return nil
		}
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	favorite.CreatedAt = e.CreatedAt
	return nil
}

func (r *FavoriteRepo) RemoveFavorite(ctx context.Context, userID, targetType, targetID string) error {
	res := r.helper.DB.WithContext(ctx).
		Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).
		Delete(&efavorite.Favorite{})
	if res.Error != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, res.Error)
	}
	return nil
}

func (r *FavoriteRepo) IsFavorite(ctx context.Context, userID, targetType, targetID string) (bool, error) {
	return r.helper.Exists(ctx, &efavorite.Favorite{}, "user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID)
}

func (r *FavoriteRepo) ListFavoriteTargets(ctx context.Context, userID, targetType string) (map[string]bool, error) {
	var es []efavorite.Favorite
	if err := r.helper.Find(ctx, &es, "user_id = ? AND target_type = ?", userID, targetType); err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(es))
	for i := range es {
		result[es[i].TargetID] = true
	}
	return result, nil
}

func (r *FavoriteRepo) DeleteUserFavorites(ctx context.Context, userID string) error {
	err := r.helper.DB.WithContext(ctx).Where("user_id = ?", userID).Delete(&efavorite.Favorite{}).Error
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return nil
}

var _ repository.FavoriteRepository = (*FavoriteRepo)(nil)
