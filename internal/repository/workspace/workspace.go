package workspace

import (
	"context"
	"errors"
	"time"

	mfavorite "github.com/tandem/tandem/internal/domain/models/favorite"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	eboard "github.com/tandem/tandem/internal/repository/entity/board"
	efavorite "github.com/tandem/tandem/internal/repository/entity/favorite"
	etask "github.com/tandem/tandem/internal/repository/entity/task"
	eworkspace "github.com/tandem/tandem/internal/repository/entity/workspace"
	"github.com/tandem/tandem/internal/repository/postgres"
	"gorm.io/gorm"
)

type WorkspaceRepo struct {
	db *gorm.DB
}

func NewWorkspaceRepo(db *gorm.DB) *WorkspaceRepo {
	return &WorkspaceRepo{db: db}
}

func (r *WorkspaceRepo) CreateWorkspace(ctx context.Context, ws *mworkspace.Workspace) error {
	e := workspaceToEntity(ws)
	err := r.db.WithContext(ctx).Create(e).Error
	if err != nil {
		if postgres.IsUniqueViolation(err) {
			return pkgerrors.Wrap(pkgerrors.ErrConflict, err)
		}
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	ws.CreatedAt = e.CreatedAt
	ws.UpdatedAt = e.UpdatedAt
	return nil
}

func (r *WorkspaceRepo) FindWorkspaceByID(ctx context.Context, id string) (*mworkspace.Workspace, error) {
	var e eworkspace.Workspace
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, err)
	}
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return workspaceToDomain(&e), nil
}

func (r *WorkspaceRepo) FindWorkspaceByInvite(ctx context.Context, token string) (*mworkspace.Workspace, error) {
	var e eworkspace.Workspace
	err := r.db.WithContext(ctx).Where("invite_token = ? AND invite_token <> ''", token).First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, err)
	}
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return workspaceToDomain(&e), nil
}

func (r *WorkspaceRepo) UpdateWorkspace(ctx context.Context, ws *mworkspace.Workspace) error {
	e := workspaceToEntity(ws)
	err := r.db.WithContext(ctx).Model(&eworkspace.Workspace{}).Where("id = ?", e.ID).Updates(map[string]interface{}{
		"name":         e.Name,
		"description":  e.Description,
		"prefix":       e.Prefix,
		"theme":        e.Theme,
		"invite_token": e.InviteToken,
	}).Error
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return nil
}

func (r *WorkspaceRepo) UpdateInvite(ctx context.Context, workspaceID, token string, expiresAt *time.Time) error {
	res := r.db.WithContext(ctx).Model(&eworkspace.Workspace{}).
		Where("id = ?", workspaceID).
		Updates(map[string]interface{}{
			"invite_token":      token,
			"invite_expires_at": expiresAt,
		})
	if res.Error != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	return nil
}

func (r *WorkspaceRepo) DeleteWorkspace(ctx context.Context, id string) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("workspace_id = ?", id).Delete(&eworkspace.WorkspaceMember{}).Error; err != nil {
			return err
		}
		if err := tx.Where("task_id IN (SELECT tasks.id FROM tasks JOIN board_columns ON board_columns.id = tasks.column_id JOIN boards ON boards.id = board_columns.board_id WHERE boards.workspace_id = ?)", id).Delete(&etask.TaskAttachment{}).Error; err != nil {
			return err
		}
		if err := tx.Where("column_id IN (SELECT board_columns.id FROM board_columns JOIN boards ON boards.id = board_columns.board_id WHERE boards.workspace_id = ?)", id).Delete(&eboard.Task{}).Error; err != nil {
			return err
		}
		if err := tx.Where("board_id IN (SELECT id FROM boards WHERE workspace_id = ?)", id).Delete(&eboard.Column{}).Error; err != nil {
			return err
		}
		if err := tx.Where("target_type = ? AND target_id = ?", mfavorite.FavoriteWorkspace, id).Delete(&efavorite.Favorite{}).Error; err != nil {
			return err
		}
		if err := tx.Where("target_type = ? AND target_id IN (SELECT id FROM boards WHERE workspace_id = ?)", mfavorite.FavoriteBoard, id).Delete(&efavorite.Favorite{}).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ?", id).Delete(&eboard.Board{}).Error; err != nil {
			return err
		}
		res := tx.Where("id = ?", id).Delete(&eworkspace.Workspace{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, err)
	}
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return nil
}

func (r *WorkspaceRepo) ListWorkspacesForUser(ctx context.Context, userID string) ([]mworkspace.WorkspaceMembership, error) {
	var rows []struct {
		eworkspace.Workspace
		Role string
	}
	err := r.db.WithContext(ctx).
		Table("workspace_members").
		Select("workspaces.*, workspace_members.role").
		Joins("JOIN workspaces ON workspaces.id = workspace_members.workspace_id").
		Where("workspace_members.user_id = ?", userID).
		Order("workspaces.created_at ASC").
		Find(&rows).Error
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	memberships := make([]mworkspace.WorkspaceMembership, 0, len(rows))
	for i := range rows {
		memberships = append(memberships, mworkspace.WorkspaceMembership{
			Workspace: *workspaceToDomain(&rows[i].Workspace),
			Role:      mworkspace.WorkspaceRole(rows[i].Role),
		})
	}
	return memberships, nil
}

func (r *WorkspaceRepo) AddMember(ctx context.Context, workspaceID, userID string, role mworkspace.WorkspaceRole) error {
	e := &eworkspace.WorkspaceMember{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        string(role),
	}
	err := r.db.WithContext(ctx).Create(e).Error
	if err != nil {
		if postgres.IsUniqueViolation(err) {
			return pkgerrors.Wrap(pkgerrors.ErrConflict, err)
		}
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return nil
}

func (r *WorkspaceRepo) FindMember(ctx context.Context, workspaceID, userID string) (*mworkspace.WorkspaceMember, error) {
	var e eworkspace.WorkspaceMember
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND user_id = ?", workspaceID, userID).First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, err)
	}
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return &mworkspace.WorkspaceMember{
		WorkspaceID: e.WorkspaceID,
		UserID:      e.UserID,
		Role:        mworkspace.WorkspaceRole(e.Role),
		CreatedAt:   e.CreatedAt,
	}, nil
}

func (r *WorkspaceRepo) RemoveMember(ctx context.Context, workspaceID, userID string) error {
	res := r.db.WithContext(ctx).Where("workspace_id = ? AND user_id = ?", workspaceID, userID).Delete(&eworkspace.WorkspaceMember{})
	if res.Error != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	return nil
}

func (r *WorkspaceRepo) UpdateMemberRole(ctx context.Context, workspaceID, userID string, role mworkspace.WorkspaceRole) error {
	res := r.db.WithContext(ctx).Model(&eworkspace.WorkspaceMember{}).
		Where("workspace_id = ? AND user_id = ?", workspaceID, userID).
		Update("role", string(role))
	if res.Error != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	return nil
}

func (r *WorkspaceRepo) ListMembers(ctx context.Context, workspaceID string) ([]mworkspace.WorkspaceMember, error) {
	var es []eworkspace.WorkspaceMember
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Order("created_at ASC").Find(&es).Error
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	members := make([]mworkspace.WorkspaceMember, 0, len(es))
	for i := range es {
		members = append(members, mworkspace.WorkspaceMember{
			WorkspaceID: es[i].WorkspaceID,
			UserID:      es[i].UserID,
			Role:        mworkspace.WorkspaceRole(es[i].Role),
			CreatedAt:   es[i].CreatedAt,
		})
	}
	return members, nil
}

func (r *WorkspaceRepo) DeleteMembersByWorkspace(ctx context.Context, workspaceID string) error {
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Delete(&eworkspace.WorkspaceMember{}).Error
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return nil
}

func (r *WorkspaceRepo) DeleteMembersByUser(ctx context.Context, userID string) error {
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&eworkspace.WorkspaceMember{}).Error
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return nil
}

func (r *WorkspaceRepo) TransferOwnership(ctx context.Context, workspaceID, oldOwnerID, newOwnerID string) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		oldRes := tx.Model(&eworkspace.WorkspaceMember{}).
			Where("workspace_id = ? AND user_id = ?", workspaceID, oldOwnerID).
			Update("role", string(mworkspace.RoleEditor))
		if oldRes.Error != nil {
			return oldRes.Error
		}
		if oldRes.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		newRes := tx.Model(&eworkspace.WorkspaceMember{}).
			Where("workspace_id = ? AND user_id = ?", workspaceID, newOwnerID).
			Update("role", string(mworkspace.RoleOwner))
		if newRes.Error != nil {
			return newRes.Error
		}
		if newRes.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, err)
	}
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return nil
}

func workspaceToEntity(ws *mworkspace.Workspace) *eworkspace.Workspace {
	return &eworkspace.Workspace{
		ID:              ws.ID,
		Name:            ws.Name,
		Description:     ws.Description,
		Prefix:          ws.Prefix,
		Theme:           ws.Theme,
		InviteToken:     ws.InviteToken,
		InviteExpiresAt: ws.InviteExpiresAt,
		CreatedAt:       ws.CreatedAt,
		UpdatedAt:       ws.UpdatedAt,
	}
}

func workspaceToDomain(e *eworkspace.Workspace) *mworkspace.Workspace {
	return &mworkspace.Workspace{
		ID:              e.ID,
		Name:            e.Name,
		Description:     e.Description,
		Prefix:          e.Prefix,
		Theme:           e.Theme,
		InviteToken:     e.InviteToken,
		InviteExpiresAt: e.InviteExpiresAt,
		CreatedAt:       e.CreatedAt,
		UpdatedAt:       e.UpdatedAt,
	}
}

var _ repository.WorkspaceRepository = (*WorkspaceRepo)(nil)
