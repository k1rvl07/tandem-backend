package workspace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/tandem/tandem/internal/domain/models"
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	"github.com/tandem/tandem/internal/http/dto"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	"github.com/tandem/tandem/internal/usecase/cacheutil"
	file "github.com/tandem/tandem/internal/usecase/file"
	"go.uber.org/zap"
)

const (
	maxDescriptionLen = 400
	inviteTTL         = 7 * 24 * time.Hour
)

const eventWorkspaceUpdated = "workspace.updated"
const eventMemberKicked = "workspace.kicked"

type UseCase interface {
	Create(ctx context.Context, actorID string, req dto.CreateWorkspaceRequest) (*dto.WorkspaceResponse, error)
	List(ctx context.Context, actorID string) ([]dto.WorkspaceResponse, error)
	Get(ctx context.Context, actorID, workspaceID string) (*dto.WorkspaceDetailResponse, error)
	Update(ctx context.Context, actorID, workspaceID string, req dto.UpdateWorkspaceRequest) (*dto.WorkspaceResponse, error)
	Delete(ctx context.Context, actorID, workspaceID string) error
	SetTheme(ctx context.Context, actorID, workspaceID, theme string) (*dto.WorkspaceResponse, error)
	AddMember(ctx context.Context, actorID, workspaceID string, req dto.AddMemberRequest) (*dto.WorkspaceMemberResponse, error)
	UpdateRole(ctx context.Context, actorID, workspaceID, targetID string, req dto.UpdateMemberRoleRequest) (*dto.WorkspaceMemberResponse, error)
	RemoveMember(ctx context.Context, actorID, workspaceID, targetID string) error
	TransferOwner(ctx context.Context, actorID, workspaceID string, req dto.TransferOwnerRequest) error
	GetInvite(ctx context.Context, actorID, workspaceID string) (*dto.WorkspaceInviteResponse, error)
	DisableInvite(ctx context.Context, actorID, workspaceID string) error
	JoinByInvite(ctx context.Context, actorID, token string) (*dto.WorkspaceResponse, error)
}

type Service struct {
	workspaces repository.WorkspaceRepository
	users      repository.UserRepository
	favorites  repository.FavoriteRepository
	boards     repository.BoardRepository
	columns    repository.ColumnRepository
	tasks      repository.TaskRepository
	files      *file.Service
	hub        ws.Hub
	cache      cache.Cache
	logger     *zap.Logger
}

func NewService(
	workspaces repository.WorkspaceRepository,
	users repository.UserRepository,
	favorites repository.FavoriteRepository,
	boards repository.BoardRepository,
	columns repository.ColumnRepository,
	tasks repository.TaskRepository,
	files *file.Service,
	hub ws.Hub,
	cache cache.Cache,
	logger *zap.Logger,
) *Service {
	return &Service{workspaces: workspaces, users: users, favorites: favorites, boards: boards, columns: columns, tasks: tasks, files: files, hub: hub, cache: cache, logger: logger}
}

func (s *Service) Create(ctx context.Context, actorID string, req dto.CreateWorkspaceRequest) (*dto.WorkspaceResponse, error) {
	if err := validateWorkspacePayload(req.Name, req.Description); err != nil {
		return nil, err
	}
	prefix, err := normalizePrefix(req.Prefix, req.Name)
	if err != nil {
		return nil, err
	}

	ws := &models.Workspace{
		ID:          uuid.New().String(),
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		Prefix:      prefix,
		Theme:       models.DefaultWorkspaceColor,
	}
	if err := s.workspaces.CreateWorkspace(ctx, ws); err != nil {
		return nil, err
	}
	if err := s.workspaces.AddMember(ctx, ws.ID, actorID, models.RoleOwner); err != nil {
		_ = s.workspaces.DeleteWorkspace(ctx, ws.ID)
		return nil, err
	}
	board, err := s.createMainBoard(ctx, ws.ID)
	if err != nil {
		if board != nil {
			_ = s.boards.DeleteBoard(ctx, board.ID)
		}
		_ = s.workspaces.DeleteWorkspace(ctx, ws.ID)
		return nil, err
	}
	ownerUser, err := s.users.FindByID(ctx, actorID)
	if err != nil {
		return nil, err
	}
	resp := workspaceToResponse(ws, models.RoleOwner, false)
	resp.Owner = memberToResponse(ownerUser, models.RoleOwner)
	cacheutil.Bump(ctx, s.cache, cacheutil.UVerKey+actorID)
	return resp, nil
}

func (s *Service) createMainBoard(ctx context.Context, workspaceID string) (*models.Board, error) {
	board := &models.Board{
		ID:          uuid.New().String(),
		WorkspaceID: workspaceID,
		Name:        "Main",
		Position:    0,
		IsMain:      true,
	}
	if err := s.boards.CreateBoard(ctx, board); err != nil {
		return nil, err
	}
	for i, name := range models.DefaultColumnNames {
		column := &models.Column{
			ID:       uuid.New().String(),
			BoardID:  board.ID,
			Name:     name,
			Position: i,
		}
		if err := s.columns.CreateColumn(ctx, column); err != nil {
			return board, err
		}
	}
	return board, nil
}

func (s *Service) List(ctx context.Context, actorID string) ([]dto.WorkspaceResponse, error) {
	uver := cacheutil.Version(ctx, s.cache, cacheutil.UVerKey+actorID)
	listKey := fmt.Sprintf("u:%s:t:v1:wslist:%s", actorID, uver)
	var cached []dto.WorkspaceResponse
	if cacheutil.Load(ctx, s.cache, listKey, &cached) {
		return cached, nil
	}
	memberships, err := s.workspaces.ListWorkspacesForUser(ctx, actorID)
	if err != nil {
		return nil, err
	}
	favorites, err := s.favorites.ListFavoriteTargets(ctx, actorID, models.FavoriteWorkspace)
	if err != nil {
		return nil, err
	}
	responses := make([]dto.WorkspaceResponse, 0, len(memberships))
	for i := range memberships {
		resp := workspaceToResponse(&memberships[i].Workspace, memberships[i].Role, favorites[memberships[i].Workspace.ID])
		owner, err := s.resolveOwner(ctx, memberships[i].Workspace.ID)
		if err != nil {
			return nil, err
		}
		resp.Owner = owner
		responses = append(responses, *resp)
	}
	cacheutil.Store(ctx, s.cache, listKey, responses, cacheutil.TTL)
	return responses, nil
}

func (s *Service) Get(ctx context.Context, actorID, workspaceID string) (*dto.WorkspaceDetailResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	wsver := cacheutil.Version(ctx, s.cache, cacheutil.WSVerKey+workspaceID)
	uver := cacheutil.Version(ctx, s.cache, cacheutil.UVerKey+actorID)
	detailKey := fmt.Sprintf("u:%s:t:v1:ws:%s:%s:%s", actorID, workspaceID, wsver, uver)
	var cached dto.WorkspaceDetailResponse
	if cacheutil.Load(ctx, s.cache, detailKey, &cached) {
		return &cached, nil
	}
	ws, err := s.workspaces.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	member, err := s.memberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	isFavorite, err := s.favorites.IsFavorite(ctx, actorID, models.FavoriteWorkspace, workspaceID)
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
			if errorsIsNotFound(err) {
				continue
			}
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
	detail := &dto.WorkspaceDetailResponse{
		ID:          ws.ID,
		Name:        ws.Name,
		Description: ws.Description,
		Prefix:      ws.Prefix,
		Theme:       ws.Theme,
		Role:        string(member.Role),
		IsFavorite:  isFavorite,
		CreatedAt:   ws.CreatedAt,
		UpdatedAt:   ws.UpdatedAt,
		Members:     memberResponses,
	}
	cacheutil.Store(ctx, s.cache, detailKey, detail, cacheutil.TTL)
	return detail, nil
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
	prefix, err := normalizePrefix(req.Prefix, req.Name)
	if err != nil {
		return nil, err
	}
	wsModel, err := s.workspaces.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	wsModel.Name = strings.TrimSpace(req.Name)
	wsModel.Description = strings.TrimSpace(req.Description)
	wsModel.Prefix = prefix
	if err := s.workspaces.UpdateWorkspace(ctx, wsModel); err != nil {
		return nil, err
	}
	response := workspaceToResponse(wsModel, member.Role, false)
	s.bumpWorkspace(ctx, workspaceID)
	s.bumpMembers(ctx, workspaceID)
	s.hub.BroadcastToRoom(workspaceRoom(workspaceID), &ws.Message{Type: eventWorkspaceUpdated, Data: response})
	return response, nil
}

func (s *Service) SetTheme(ctx context.Context, actorID, workspaceID, theme string) (*dto.WorkspaceResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	theme = strings.TrimSpace(theme)
	if err := validate.Color(theme); err != nil {
		return nil, err
	}
	member, err := s.memberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	if member.Role != models.RoleOwner && member.Role != models.RoleEditor {
		return nil, pkgerrors.ErrForbidden
	}
	wsModel, err := s.workspaces.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	wsModel.Theme = theme
	if err := s.workspaces.UpdateWorkspace(ctx, wsModel); err != nil {
		return nil, err
	}
	response := workspaceToResponse(wsModel, member.Role, false)
	s.bumpWorkspace(ctx, workspaceID)
	s.bumpMembers(ctx, workspaceID)
	s.hub.BroadcastToRoom(workspaceRoom(workspaceID), &ws.Message{Type: eventWorkspaceUpdated, Data: response})
	return response, nil
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
	s.bumpMembers(ctx, workspaceID)
	keys, err := s.tasks.CollectWorkspaceKeys(ctx, workspaceID)
	if err != nil {
		return err
	}
	s.files.RemoveMany(ctx, keys)
	if err := s.workspaces.DeleteWorkspace(ctx, workspaceID); err != nil {
		return err
	}
	s.bumpWorkspace(ctx, workspaceID)
	s.hub.BroadcastToRoom(workspaceRoom(workspaceID), &ws.Message{
		Type: eventMemberKicked,
		Data: map[string]string{"workspace_id": workspaceID},
	})
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

	role := models.RoleMember
	switch models.WorkspaceRole(req.Role) {
	case "":
	case models.RoleEditor:
		role = models.RoleEditor
	case models.RoleMember:
		role = models.RoleMember
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
	s.bumpWorkspace(ctx, workspaceID)
	s.bumpUser(ctx, target.ID)
	s.broadcastUpdated(ctx, workspaceID, member.Role)
	return memberToResponse(target, role), nil
}

func (s *Service) UpdateRole(ctx context.Context, actorID, workspaceID, targetID string, req dto.UpdateMemberRoleRequest) (*dto.WorkspaceMemberResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	if err := validate.UUID(targetID); err != nil {
		return nil, err
	}
	member, err := s.memberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	if member.Role != models.RoleOwner {
		return nil, pkgerrors.ErrForbidden
	}
	role := models.WorkspaceRole(req.Role)
	if role != models.RoleEditor && role != models.RoleMember {
		return nil, pkgerrors.NewValidationError("role must be editor or member")
	}
	target, err := s.workspaces.FindMember(ctx, workspaceID, targetID)
	if err != nil {
		return nil, err
	}
	if target.Role == models.RoleOwner {
		return nil, pkgerrors.NewValidationError("cannot change the owner's role; transfer ownership instead")
	}
	if err := s.workspaces.UpdateMemberRole(ctx, workspaceID, targetID, role); err != nil {
		return nil, err
	}
	user, err := s.users.FindByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	s.bumpWorkspace(ctx, workspaceID)
	s.bumpUser(ctx, targetID)
	s.broadcastUpdated(ctx, workspaceID, member.Role)
	return memberToResponse(user, role), nil
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
	if err := s.workspaces.RemoveMember(ctx, workspaceID, targetID); err != nil {
		return err
	}
	s.bumpWorkspace(ctx, workspaceID)
	s.bumpUser(ctx, targetID)
	s.broadcastUpdated(ctx, workspaceID, member.Role)
	s.hub.BroadcastToRoom(workspaceRoom(workspaceID), &ws.Message{
		Type: eventMemberKicked,
		Data: map[string]string{"workspace_id": workspaceID, "user_id": targetID},
	})
	return nil
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
	if err := s.workspaces.TransferOwnership(ctx, workspaceID, actorID, target.UserID); err != nil {
		return err
	}
	s.bumpWorkspace(ctx, workspaceID)
	s.bumpUser(ctx, target.UserID)
	s.bumpUser(ctx, actorID)
	s.broadcastUpdated(ctx, workspaceID, models.RoleEditor)
	return nil
}

func (s *Service) GetInvite(ctx context.Context, actorID, workspaceID string) (*dto.WorkspaceInviteResponse, error) {
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
	ws, err := s.workspaces.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if ws.InviteToken == "" || ws.InviteExpiresAt == nil || ws.InviteExpiresAt.Before(time.Now()) {
		ws.InviteToken = uuid.New().String()
		expiresAt := time.Now().Add(inviteTTL)
		ws.InviteExpiresAt = &expiresAt
		if err := s.workspaces.UpdateInvite(ctx, ws.ID, ws.InviteToken, ws.InviteExpiresAt); err != nil {
			return nil, err
		}
	}
	return &dto.WorkspaceInviteResponse{InviteToken: &ws.InviteToken, ExpiresAt: ws.InviteExpiresAt}, nil
}

func (s *Service) DisableInvite(ctx context.Context, actorID, workspaceID string) error {
	if err := validate.UUID(workspaceID); err != nil {
		return err
	}
	member, err := s.memberOf(ctx, actorID, workspaceID)
	if err != nil {
		return err
	}
	if member.Role != models.RoleOwner && member.Role != models.RoleEditor {
		return pkgerrors.ErrForbidden
	}
	if err := s.workspaces.UpdateInvite(ctx, workspaceID, "", nil); err != nil {
		return err
	}
	return nil
}

func (s *Service) JoinByInvite(ctx context.Context, actorID, token string) (*dto.WorkspaceResponse, error) {
	if token == "" {
		return nil, pkgerrors.NewValidationError("invite token is required")
	}
	ws, err := s.workspaces.FindWorkspaceByInvite(ctx, token)
	if err != nil {
		return nil, err
	}
	if ws.InviteExpiresAt != nil && ws.InviteExpiresAt.Before(time.Now()) {
		if err := s.workspaces.UpdateInvite(ctx, ws.ID, "", nil); err != nil {
			s.logger.Warn("clear expired invite failed", zap.String("workspace_id", ws.ID), zap.Error(err))
		}
		return nil, pkgerrors.NewValidationError("invite link has expired")
	}
	member, findErr := s.workspaces.FindMember(ctx, ws.ID, actorID)
	if findErr != nil {
		if !errorsIsNotFound(findErr) {
			return nil, findErr
		}
		member = nil
	}
	role := models.RoleMember
	added := false
	if member != nil {
		role = member.Role
	} else {
		if err := s.workspaces.AddMember(ctx, ws.ID, actorID, models.RoleMember); err != nil {
			return nil, err
		}
		added = true
	}
	owner, err := s.resolveOwner(ctx, ws.ID)
	if err != nil {
		return nil, err
	}
	resp := workspaceToResponse(ws, role, false)
	resp.Owner = owner
	if added {
		s.bumpWorkspace(ctx, ws.ID)
		s.bumpUser(ctx, actorID)
		s.broadcastUpdated(ctx, ws.ID, role)
	}
	if err := s.workspaces.UpdateInvite(ctx, ws.ID, "", nil); err != nil {
		s.logger.Warn("invalidate invite failed", zap.String("workspace_id", ws.ID), zap.Error(err))
	}
	return resp, nil
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

func (s *Service) bumpWorkspace(ctx context.Context, workspaceID string) {
	cacheutil.Bump(ctx, s.cache, cacheutil.WSVerKey+workspaceID)
}

func (s *Service) bumpUser(ctx context.Context, userID string) {
	cacheutil.Bump(ctx, s.cache, cacheutil.UVerKey+userID)
}

func (s *Service) bumpMembers(ctx context.Context, workspaceID string) {
	members, err := s.workspaces.ListMembers(ctx, workspaceID)
	if err != nil {
		return
	}
	for i := range members {
		s.bumpUser(ctx, members[i].UserID)
	}
}

func (s *Service) resolveOwner(ctx context.Context, workspaceID string) (*dto.WorkspaceMemberResponse, error) {
	members, err := s.workspaces.ListMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	for i := range members {
		if members[i].Role == models.RoleOwner {
			user, err := s.users.FindByID(ctx, members[i].UserID)
			if err != nil {
				if errorsIsNotFound(err) {
					continue
				}
				return nil, err
			}
			return memberToResponse(user, members[i].Role), nil
		}
	}
	return nil, nil
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

func normalizePrefix(prefix, name string) (string, error) {
	prefix = strings.ToUpper(strings.TrimSpace(prefix))
	if prefix == "" {
		var b strings.Builder
		for _, r := range strings.ToUpper(name) {
			if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
				b.WriteRune(r)
				if b.Len() == 3 {
					break
				}
			}
		}
		prefix = b.String()
	}
	if prefix == "" {
		prefix = "WS"
	}
	if err := validate.Prefix(prefix); err != nil {
		return "", err
	}
	return prefix, nil
}

func workspaceToResponse(ws *models.Workspace, role models.WorkspaceRole, isFavorite bool) *dto.WorkspaceResponse {
	return &dto.WorkspaceResponse{
		ID:          ws.ID,
		Name:        ws.Name,
		Description: ws.Description,
		Prefix:      ws.Prefix,
		Theme:       ws.Theme,
		Role:        string(role),
		IsFavorite:  isFavorite,
		CreatedAt:   ws.CreatedAt,
		UpdatedAt:   ws.UpdatedAt,
	}
}

func (s *Service) broadcastUpdated(ctx context.Context, workspaceID string, role models.WorkspaceRole) {
	wsModel, err := s.workspaces.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return
	}
	s.hub.BroadcastToRoom(workspaceRoom(workspaceID), &ws.Message{Type: eventWorkspaceUpdated, Data: workspaceToResponse(wsModel, role, false)})
}

func workspaceRoom(workspaceID string) string {
	return "workspace:" + workspaceID
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
