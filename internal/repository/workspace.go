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

type WorkspaceRepo struct {
	db *gorm.DB
}

func NewWorkspaceRepo(db *gorm.DB) *WorkspaceRepo {
	return &WorkspaceRepo{db: db}
}

func (r *WorkspaceRepo) CreateWorkspace(ctx context.Context, ws *models.Workspace) error {
	e := workspaceToEntity(ws)
	err := r.db.WithContext(ctx).Create(e).Error
	if err != nil {
		if isUniqueViolation(err) {
			return pkgerrors.Wrap(pkgerrors.ErrConflict, err)
		}
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	ws.CreatedAt = e.CreatedAt
	ws.UpdatedAt = e.UpdatedAt
	return nil
}

func (r *WorkspaceRepo) FindWorkspaceByID(ctx context.Context, id string) (*models.Workspace, error) {
	var e entity.Workspace
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, err)
	}
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return workspaceToDomain(&e), nil
}

func (r *WorkspaceRepo) UpdateWorkspace(ctx context.Context, ws *models.Workspace) error {
	e := workspaceToEntity(ws)
	err := r.db.WithContext(ctx).Model(&entity.Workspace{}).Where("id = ?", e.ID).Updates(map[string]interface{}{
		"name":        e.Name,
		"description": e.Description,
	}).Error
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return nil
}

func (r *WorkspaceRepo) DeleteWorkspace(ctx context.Context, id string) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("workspace_id = ?", id).Delete(&entity.WorkspaceMember{}).Error; err != nil {
			return err
		}
		res := tx.Where("id = ?", id).Delete(&entity.Workspace{})
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

func (r *WorkspaceRepo) ListWorkspacesForUser(ctx context.Context, userID string) ([]models.WorkspaceMembership, error) {
	var rows []struct {
		entity.Workspace
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
	memberships := make([]models.WorkspaceMembership, 0, len(rows))
	for i := range rows {
		memberships = append(memberships, models.WorkspaceMembership{
			Workspace: *workspaceToDomain(&rows[i].Workspace),
			Role:      models.WorkspaceRole(rows[i].Role),
		})
	}
	return memberships, nil
}

func (r *WorkspaceRepo) AddMember(ctx context.Context, workspaceID, userID string, role models.WorkspaceRole) error {
	e := &entity.WorkspaceMember{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        string(role),
	}
	err := r.db.WithContext(ctx).Create(e).Error
	if err != nil {
		if isUniqueViolation(err) {
			return pkgerrors.Wrap(pkgerrors.ErrConflict, err)
		}
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return nil
}

func (r *WorkspaceRepo) FindMember(ctx context.Context, workspaceID, userID string) (*models.WorkspaceMember, error) {
	var e entity.WorkspaceMember
	err := r.db.WithContext(ctx).Where("workspace_id = ? AND user_id = ?", workspaceID, userID).First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, err)
	}
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return &models.WorkspaceMember{
		WorkspaceID: e.WorkspaceID,
		UserID:      e.UserID,
		Role:        models.WorkspaceRole(e.Role),
		CreatedAt:   e.CreatedAt,
	}, nil
}

func (r *WorkspaceRepo) RemoveMember(ctx context.Context, workspaceID, userID string) error {
	res := r.db.WithContext(ctx).Where("workspace_id = ? AND user_id = ?", workspaceID, userID).Delete(&entity.WorkspaceMember{})
	if res.Error != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	return nil
}

func (r *WorkspaceRepo) UpdateMemberRole(ctx context.Context, workspaceID, userID string, role models.WorkspaceRole) error {
	res := r.db.WithContext(ctx).Model(&entity.WorkspaceMember{}).
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

func (r *WorkspaceRepo) ListMembers(ctx context.Context, workspaceID string) ([]models.WorkspaceMember, error) {
	var es []entity.WorkspaceMember
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Order("created_at ASC").Find(&es).Error
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	members := make([]models.WorkspaceMember, 0, len(es))
	for i := range es {
		members = append(members, models.WorkspaceMember{
			WorkspaceID: es[i].WorkspaceID,
			UserID:      es[i].UserID,
			Role:        models.WorkspaceRole(es[i].Role),
			CreatedAt:   es[i].CreatedAt,
		})
	}
	return members, nil
}

func (r *WorkspaceRepo) DeleteMembersByWorkspace(ctx context.Context, workspaceID string) error {
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Delete(&entity.WorkspaceMember{}).Error
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return nil
}

func workspaceToEntity(ws *models.Workspace) *entity.Workspace {
	return &entity.Workspace{
		ID:          ws.ID,
		Name:        ws.Name,
		Description: ws.Description,
		CreatedAt:   ws.CreatedAt,
		UpdatedAt:   ws.UpdatedAt,
	}
}

func workspaceToDomain(e *entity.Workspace) *models.Workspace {
	return &models.Workspace{
		ID:          e.ID,
		Name:        e.Name,
		Description: e.Description,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

var _ repository.WorkspaceRepository = (*WorkspaceRepo)(nil)
