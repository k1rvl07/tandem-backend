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
	"gorm.io/gorm/clause"
)

type WorkspaceRepo struct {
	helper *postgres.Helper
}

func NewWorkspaceRepo(db *gorm.DB) *WorkspaceRepo {
	return &WorkspaceRepo{helper: postgres.NewHelper(db)}
}

func (r *WorkspaceRepo) CreateWorkspace(ctx context.Context, ws *mworkspace.Workspace) error {
	e := workspaceToEntity(ws)
	err := r.helper.DB.WithContext(ctx).Create(e).Error
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
	if err := r.helper.First(ctx, &e, "id = ?", id); err != nil {
		return nil, err
	}
	return workspaceToDomain(&e), nil
}

func (r *WorkspaceRepo) FindWorkspaceByInvite(ctx context.Context, token string) (*mworkspace.Workspace, error) {
	var e eworkspace.Workspace
	if err := r.helper.First(ctx, &e, "invite_token = ? AND invite_token <> ''", token); err != nil {
		return nil, err
	}
	return workspaceToDomain(&e), nil
}

func (r *WorkspaceRepo) UpdateWorkspace(ctx context.Context, ws *mworkspace.Workspace) error {
	e := workspaceToEntity(ws)
	err := r.helper.DB.WithContext(ctx).Model(&eworkspace.Workspace{}).Where("id = ?", e.ID).Updates(map[string]interface{}{
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
	res := r.helper.DB.WithContext(ctx).Model(&eworkspace.Workspace{}).
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

func (r *WorkspaceRepo) ClaimInvite(ctx context.Context, workspaceID, token string) (bool, error) {
	res := r.helper.DB.WithContext(ctx).Model(&eworkspace.Workspace{}).
		Where("id = ? AND invite_token = ? AND invite_token <> ''", workspaceID, token).
		Updates(map[string]interface{}{
			"invite_token":      "",
			"invite_expires_at": nil,
		})
	if res.Error != nil {
		return false, pkgerrors.Wrap(pkgerrors.ErrInternal, res.Error)
	}
	return res.RowsAffected == 1, nil
}

func (r *WorkspaceRepo) ClaimInviteAndAddMember(ctx context.Context, workspaceID, token, userID string, role mworkspace.WorkspaceRole) (bool, bool, error) {
	var claimed, added bool
	err := r.helper.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var memberCount int64
		if err := tx.Model(&eworkspace.WorkspaceMember{}).
			Where("workspace_id = ? AND user_id = ?", workspaceID, userID).
			Count(&memberCount).Error; err != nil {
			return err
		}
		if memberCount > 0 {
			claimed = true
			return nil
		}
		res := tx.Model(&eworkspace.Workspace{}).
			Where("id = ? AND invite_token = ? AND invite_token <> ''", workspaceID, token).
			Updates(map[string]interface{}{
				"invite_token":      "",
				"invite_expires_at": nil,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return nil
		}
		claimed = true
		ins := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&eworkspace.WorkspaceMember{
			WorkspaceID: workspaceID,
			UserID:      userID,
			Role:        string(role),
		})
		if ins.Error != nil {
			return ins.Error
		}
		added = ins.RowsAffected == 1
		return nil
	})
	if err != nil {
		return false, false, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return claimed, added, nil
}

func (r *WorkspaceRepo) DeleteWorkspace(ctx context.Context, id string) error {
	err := r.helper.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
	err := r.helper.DB.WithContext(ctx).
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
	err := r.helper.DB.WithContext(ctx).Create(e).Error
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
	if err := r.helper.First(ctx, &e, "workspace_id = ? AND user_id = ?", workspaceID, userID); err != nil {
		return nil, err
	}
	return &mworkspace.WorkspaceMember{
		WorkspaceID: e.WorkspaceID,
		UserID:      e.UserID,
		Role:        mworkspace.WorkspaceRole(e.Role),
		CreatedAt:   e.CreatedAt,
	}, nil
}

func (r *WorkspaceRepo) RemoveMember(ctx context.Context, workspaceID, userID string) error {
	res := r.helper.DB.WithContext(ctx).Where("workspace_id = ? AND user_id = ?", workspaceID, userID).Delete(&eworkspace.WorkspaceMember{})
	if res.Error != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	return nil
}

func (r *WorkspaceRepo) UpdateMemberRole(ctx context.Context, workspaceID, userID string, role mworkspace.WorkspaceRole) error {
	res := r.helper.DB.WithContext(ctx).Model(&eworkspace.WorkspaceMember{}).
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
	if err := r.helper.List(ctx, &es, "created_at ASC", "workspace_id = ?", workspaceID); err != nil {
		return nil, err
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
	err := r.helper.DB.WithContext(ctx).Where("workspace_id = ?", workspaceID).Delete(&eworkspace.WorkspaceMember{}).Error
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return nil
}

func (r *WorkspaceRepo) DeleteMembersByUser(ctx context.Context, userID string) error {
	err := r.helper.DB.WithContext(ctx).Where("user_id = ?", userID).Delete(&eworkspace.WorkspaceMember{}).Error
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return nil
}

func (r *WorkspaceRepo) TransferOwnership(ctx context.Context, workspaceID, oldOwnerID, newOwnerID string) error {
	err := r.helper.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
