package workspace

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/tandem/tandem/internal/domain/models"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/http/dto"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
)

const maxDescriptionLen = 400

type UseCase interface {
	Create(ctx context.Context, actorID string, req dto.CreateWorkspaceRequest) (*dto.WorkspaceResponse, error)
	List(ctx context.Context, actorID string) ([]dto.WorkspaceResponse, error)
	Get(ctx context.Context, actorID, workspaceID string) (*dto.WorkspaceDetailResponse, error)
	Update(ctx context.Context, actorID, workspaceID string, req dto.UpdateWorkspaceRequest) (*dto.WorkspaceResponse, error)
	Delete(ctx context.Context, actorID, workspaceID string) error
	AddMember(ctx context.Context, actorID, workspaceID string, req dto.AddMemberRequest) (*dto.WorkspaceMemberResponse, error)
	RemoveMember(ctx context.Context, actorID, workspaceID, targetID string) error
	TransferOwner(ctx context.Context, actorID, workspaceID string, req dto.TransferOwnerRequest) error
}

type Service struct {
	workspaces repository.WorkspaceRepository
	users      repository.UserRepository
}

func NewService(workspaces repository.WorkspaceRepository, users repository.UserRepository) *Service {
	return &Service{workspaces: workspaces, users: users}
}

func (s *Service) Create(ctx context.Context, actorID string, req dto.CreateWorkspaceRequest) (*dto.WorkspaceResponse, error) {
	if err := validateWorkspacePayload(req.Name, req.Description); err != nil {
		return nil, err
	}

	ws := &models.Workspace{
		ID:          uuid.New().String(),
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
	}
	if err := s.workspaces.CreateWorkspace(ctx, ws); err != nil {
		return nil, err
	}
	if err := s.workspaces.AddMember(ctx, ws.ID, actorID, models.RoleOwner); err != nil {
		return nil, err
	}
	return workspaceToResponse(ws, models.RoleOwner), nil
}

func (s *Service) List(ctx context.Context, actorID string) ([]dto.WorkspaceResponse, error) {
	memberships, err := s.workspaces.ListWorkspacesForUser(ctx, actorID)
	if err != nil {
		return nil, err
	}
	responses := make([]dto.WorkspaceResponse, 0, len(memberships))
	for i := range memberships {
		responses = append(responses, *workspaceToResponse(&memberships[i].Workspace, memberships[i].Role))
	}
	return responses, nil
}

func (s *Service) Get(ctx context.Context, actorID, workspaceID string) (*dto.WorkspaceDetailResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	ws, err := s.workspaces.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	member, err := s.memberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	members, err := s.workspaces.ListMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	memberResponses := make([]dto.WorkspaceMemberResponse, 0, len(members))
	for i := range members {
		user, err := s.users.FindByID(ctx, members[i].UserID)
		if err != nil {
			return nil, err
		}
		memberResponses = append(memberResponses, dto.WorkspaceMemberResponse{
			ID:          user.ID,
			Login:       user.Login,
			DisplayName: user.DisplayName,
			AvatarKey:   user.AvatarKey,
			Role:        string(members[i].Role),
			JoinedAt:    members[i].CreatedAt,
		})
	}
	return &dto.WorkspaceDetailResponse{
		ID:          ws.ID,
		Name:        ws.Name,
		Description: ws.Description,
		Role:        string(member.Role),
		CreatedAt:   ws.CreatedAt,
		UpdatedAt:   ws.UpdatedAt,
		Members:     memberResponses,
	}, nil
}

func (s *Service) Update(ctx context.Context, actorID, workspaceID string, req dto.UpdateWorkspaceRequest) (*dto.WorkspaceResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	member, err := s.memberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	if member.Role != models.RoleOwner && member.Role != models.RoleEditor {
		return nil, pkgerrors.ErrForbidden
	}
	if err := validateWorkspacePayload(req.Name, req.Description); err != nil {
		return nil, err
	}
	ws, err := s.workspaces.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	ws.Name = strings.TrimSpace(req.Name)
	ws.Description = strings.TrimSpace(req.Description)
	if err := s.workspaces.UpdateWorkspace(ctx, ws); err != nil {
		return nil, err
	}
	return workspaceToResponse(ws, member.Role), nil
}

func (s *Service) Delete(ctx context.Context, actorID, workspaceID string) error {
	if err := validate.UUID(workspaceID); err != nil {
		return err
	}
	member, err := s.memberOf(ctx, actorID, workspaceID)
	if err != nil {
		return err
	}
	if member.Role != models.RoleOwner {
		return pkgerrors.ErrForbidden
	}
	if err := s.workspaces.DeleteWorkspace(ctx, workspaceID); err != nil {
		return err
	}
	return nil
}

func (s *Service) AddMember(ctx context.Context, actorID, workspaceID string, req dto.AddMemberRequest) (*dto.WorkspaceMemberResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	member, err := s.memberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	if member.Role != models.RoleOwner {
		return nil, pkgerrors.ErrForbidden
	}

	role := models.RoleViewer
	switch models.WorkspaceRole(req.Role) {
	case "":
	case models.RoleEditor:
		role = models.RoleEditor
	case models.RoleViewer:
		role = models.RoleViewer
	case models.RoleOwner:
		return nil, pkgerrors.NewValidationError("owner must be assigned via ownership transfer")
	default:
		return nil, pkgerrors.NewValidationError("invalid role")
	}

	login := validate.NormalizeLogin(req.Login)
	if err := validate.Login(login); err != nil {
		return nil, err
	}
	target, err := s.users.FindByLogin(ctx, login)
	if err != nil {
		return nil, err
	}
	if _, err := s.workspaces.FindMember(ctx, workspaceID, target.ID); err == nil {
		return nil, pkgerrors.ErrConflict
	} else if !errorsIsNotFound(err) {
		return nil, err
	}
	if err := s.workspaces.AddMember(ctx, workspaceID, target.ID, role); err != nil {
		return nil, err
	}
	return memberToResponse(target, role), nil
}

func (s *Service) RemoveMember(ctx context.Context, actorID, workspaceID, targetID string) error {
	if err := validate.UUID(workspaceID); err != nil {
		return err
	}
	if err := validate.UUID(targetID); err != nil {
		return err
	}
	member, err := s.memberOf(ctx, actorID, workspaceID)
	if err != nil {
		return err
	}
	if member.Role != models.RoleOwner {
		return pkgerrors.ErrForbidden
	}
	target, err := s.workspaces.FindMember(ctx, workspaceID, targetID)
	if err != nil {
		return err
	}
	if target.Role == models.RoleOwner {
		return pkgerrors.NewValidationError("transfer ownership before removing the owner")
	}
	return s.workspaces.RemoveMember(ctx, workspaceID, targetID)
}

func (s *Service) TransferOwner(ctx context.Context, actorID, workspaceID string, req dto.TransferOwnerRequest) error {
	if err := validate.UUID(workspaceID); err != nil {
		return err
	}
	if err := validate.UUID(req.UserID); err != nil {
		return err
	}
	member, err := s.memberOf(ctx, actorID, workspaceID)
	if err != nil {
		return err
	}
	if member.Role != models.RoleOwner {
		return pkgerrors.ErrForbidden
	}
	if req.UserID == actorID {
		return pkgerrors.NewValidationError("already the owner")
	}
	target, err := s.workspaces.FindMember(ctx, workspaceID, req.UserID)
	if err != nil {
		return err
	}
	if err := s.workspaces.UpdateMemberRole(ctx, workspaceID, target.UserID, models.RoleOwner); err != nil {
		return err
	}
	return s.workspaces.UpdateMemberRole(ctx, workspaceID, actorID, models.RoleEditor)
}

func (s *Service) memberOf(ctx context.Context, actorID, workspaceID string) (*models.WorkspaceMember, error) {
	member, err := s.workspaces.FindMember(ctx, workspaceID, actorID)
	if err != nil {
		if errorsIsNotFound(err) {
			return nil, pkgerrors.ErrForbidden
		}
		return nil, err
	}
	return member, nil
}

func validateWorkspacePayload(name, description string) error {
	if err := validate.Name(name); err != nil {
		return err
	}
	if utf8.RuneCountInString(strings.TrimSpace(description)) > maxDescriptionLen {
		return pkgerrors.NewValidationError("description must be at most %d characters", maxDescriptionLen)
	}
	return nil
}

func workspaceToResponse(ws *models.Workspace, role models.WorkspaceRole) *dto.WorkspaceResponse {
	return &dto.WorkspaceResponse{
		ID:          ws.ID,
		Name:        ws.Name,
		Description: ws.Description,
		Role:        string(role),
		CreatedAt:   ws.CreatedAt,
		UpdatedAt:   ws.UpdatedAt,
	}
}

func memberToResponse(user *models.User, role models.WorkspaceRole) *dto.WorkspaceMemberResponse {
	return &dto.WorkspaceMemberResponse{
		ID:          user.ID,
		Login:       user.Login,
		DisplayName: user.DisplayName,
		AvatarKey:   user.AvatarKey,
		Role:        string(role),
	}
}

func errorsIsNotFound(err error) bool {
	return errors.Is(err, pkgerrors.ErrNotFound)
}

var _ UseCase = (*Service)(nil)
