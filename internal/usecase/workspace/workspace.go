package workspace

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	mboard "github.com/tandem/tandem/internal/domain/models/board"
	mcolumn "github.com/tandem/tandem/internal/domain/models/column"
	mfavorite "github.com/tandem/tandem/internal/domain/models/favorite"
	muser "github.com/tandem/tandem/internal/domain/models/user"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	dworkspace "github.com/tandem/tandem/internal/http/dto/workspace"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	"github.com/tandem/tandem/internal/usecase/cacheutil"
	"github.com/tandem/tandem/internal/usecase/common"
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
	Create(ctx context.Context, actorID string, req dworkspace.CreateWorkspaceRequest) (*dworkspace.WorkspaceResponse, error)
	List(ctx context.Context, actorID string) ([]dworkspace.WorkspaceResponse, error)
	Get(ctx context.Context, actorID, workspaceID string) (*dworkspace.WorkspaceDetailResponse, error)
	Update(ctx context.Context, actorID, workspaceID string, req dworkspace.UpdateWorkspaceRequest) (*dworkspace.WorkspaceResponse, error)
	Delete(ctx context.Context, actorID, workspaceID string) error
	SetTheme(ctx context.Context, actorID, workspaceID, theme string) (*dworkspace.WorkspaceResponse, error)
	AddMember(ctx context.Context, actorID, workspaceID string, req dworkspace.AddMemberRequest) (*dworkspace.WorkspaceMemberResponse, error)
	UpdateRole(ctx context.Context, actorID, workspaceID, targetID string, req dworkspace.UpdateMemberRoleRequest) (*dworkspace.WorkspaceMemberResponse, error)
	RemoveMember(ctx context.Context, actorID, workspaceID, targetID string) error
	TransferOwner(ctx context.Context, actorID, workspaceID string, req dworkspace.TransferOwnerRequest) error
	GetInvite(ctx context.Context, actorID, workspaceID string) (*dworkspace.WorkspaceInviteResponse, error)
	DisableInvite(ctx context.Context, actorID, workspaceID string) error
	JoinByInvite(ctx context.Context, actorID, token string) (*dworkspace.WorkspaceResponse, error)
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

type Deps struct {
	Workspaces repository.WorkspaceRepository
	Users      repository.UserRepository
	Favorites  repository.FavoriteRepository
	Boards     repository.BoardRepository
	Columns    repository.ColumnRepository
	Tasks      repository.TaskRepository
	Files      *file.Service
	Hub        ws.Hub
	Cache      cache.Cache
	Logger     *zap.Logger
}

func NewService(deps Deps) *Service {
	return &Service{workspaces: deps.Workspaces, users: deps.Users, favorites: deps.Favorites, boards: deps.Boards, columns: deps.Columns, tasks: deps.Tasks, files: deps.Files, hub: deps.Hub, cache: deps.Cache, logger: deps.Logger}
}

func (s *Service) Create(ctx context.Context, actorID string, req dworkspace.CreateWorkspaceRequest) (*dworkspace.WorkspaceResponse, error) {
	if err := validateWorkspacePayload(req.Name, req.Description); err != nil {
		return nil, err
	}
	prefix, err := normalizePrefix(req.Prefix, req.Name)
	if err != nil {
		return nil, err
	}

	ws := &mworkspace.Workspace{
		ID:          uuid.New().String(),
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		Prefix:      prefix,
		Theme:       mworkspace.DefaultWorkspaceColor,
	}
	if err := s.workspaces.CreateWorkspace(ctx, ws); err != nil {
		return nil, err
	}
	if err := s.workspaces.AddMember(ctx, ws.ID, actorID, mworkspace.RoleOwner); err != nil {
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
	resp := workspaceToResponse(ws, mworkspace.RoleOwner, false)
	resp.Owner = memberToResponse(ownerUser, mworkspace.RoleOwner)
	cacheutil.Bump(ctx, s.cache, cacheutil.UVerKey+actorID)
	return resp, nil
}

func (s *Service) createMainBoard(ctx context.Context, workspaceID string) (*mboard.Board, error) {
	board := &mboard.Board{
		ID:          uuid.New().String(),
		WorkspaceID: workspaceID,
		Name:        "Main",
		Position:    0,
		IsMain:      true,
	}
	if err := s.boards.CreateBoard(ctx, board); err != nil {
		return nil, err
	}
	for i, name := range mcolumn.DefaultColumnNames {
		column := &mcolumn.Column{
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

func (s *Service) List(ctx context.Context, actorID string) ([]dworkspace.WorkspaceResponse, error) {
	uver := cacheutil.Version(ctx, s.cache, cacheutil.UVerKey+actorID)
	listKey := fmt.Sprintf("u:%s:t:v1:wslist:%s", actorID, uver)
	var cached []dworkspace.WorkspaceResponse
	if cacheutil.Load(ctx, s.cache, listKey, &cached) {
		return cached, nil
	}
	memberships, err := s.workspaces.ListWorkspacesForUser(ctx, actorID)
	if err != nil {
		return nil, err
	}
	favorites, err := s.favorites.ListFavoriteTargets(ctx, actorID, mfavorite.FavoriteWorkspace)
	if err != nil {
		return nil, err
	}
	responses := make([]dworkspace.WorkspaceResponse, 0, len(memberships))
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

func (s *Service) Get(ctx context.Context, actorID, workspaceID string) (*dworkspace.WorkspaceDetailResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	wsver := cacheutil.Version(ctx, s.cache, cacheutil.WSVerKey+workspaceID)
	uver := cacheutil.Version(ctx, s.cache, cacheutil.UVerKey+actorID)
	detailKey := fmt.Sprintf("u:%s:t:v1:ws:%s:%s:%s", actorID, workspaceID, wsver, uver)
	var cached dworkspace.WorkspaceDetailResponse
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
	isFavorite, err := s.favorites.IsFavorite(ctx, actorID, mfavorite.FavoriteWorkspace, workspaceID)
	if err != nil {
		return nil, err
	}
	members, err := s.workspaces.ListMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	memberResponses, err := s.buildMemberResponses(ctx, members)
	if err != nil {
		return nil, err
	}
	detail := &dworkspace.WorkspaceDetailResponse{
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

func (s *Service) buildMemberResponses(ctx context.Context, members []mworkspace.WorkspaceMember) ([]dworkspace.WorkspaceMemberResponse, error) {
	memberResponses := make([]dworkspace.WorkspaceMemberResponse, 0, len(members))
	for i := range members {
		user, err := s.users.FindByID(ctx, members[i].UserID)
		if err != nil {
			if common.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		memberResponses = append(memberResponses, dworkspace.WorkspaceMemberResponse{
			ID:          user.ID,
			Login:       user.Login,
			DisplayName: user.DisplayName,
			AvatarKey:   user.AvatarKey,
			Role:        string(members[i].Role),
			JoinedAt:    members[i].CreatedAt,
		})
	}
	return memberResponses, nil
}

func (s *Service) Update(ctx context.Context, actorID, workspaceID string, req dworkspace.UpdateWorkspaceRequest) (*dworkspace.WorkspaceResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	member, err := s.memberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.requireEditor(member.Role); err != nil {
		return nil, err
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

func (s *Service) SetTheme(ctx context.Context, actorID, workspaceID, theme string) (*dworkspace.WorkspaceResponse, error) {
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
	if err := s.requireEditor(member.Role); err != nil {
		return nil, err
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
	if err := s.requireOwner(member.Role); err != nil {
		return err
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

func (s *Service) AddMember(ctx context.Context, actorID, workspaceID string, req dworkspace.AddMemberRequest) (*dworkspace.WorkspaceMemberResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	member, err := s.memberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.requireOwner(member.Role); err != nil {
		return nil, err
	}

	role, err := normalizeMemberRole(mworkspace.WorkspaceRole(req.Role))
	if err != nil {
		return nil, err
	}

	login := validate.NormalizeLogin(req.Login)
	if err := validate.Login(login); err != nil {
		return nil, err
	}
	target, err := s.findUserForAdd(ctx, workspaceID, login)
	if err != nil {
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

func (s *Service) findUserForAdd(ctx context.Context, workspaceID, login string) (*muser.User, error) {
	target, err := s.users.FindByLogin(ctx, login)
	if err != nil {
		return nil, err
	}
	if _, err := s.workspaces.FindMember(ctx, workspaceID, target.ID); err == nil {
		return nil, pkgerrors.ErrConflict
	} else if !common.IsNotFound(err) {
		return nil, err
	}
	return target, nil
}

func normalizeMemberRole(role mworkspace.WorkspaceRole) (mworkspace.WorkspaceRole, error) {
	normalized := mworkspace.RoleMember
	switch role {
	case "":
	case mworkspace.RoleEditor:
		normalized = mworkspace.RoleEditor
	case mworkspace.RoleMember:
		normalized = mworkspace.RoleMember
	case mworkspace.RoleOwner:
		return mworkspace.RoleMember, pkgerrors.NewValidationError("owner must be assigned via ownership transfer")
	default:
		return mworkspace.RoleMember, pkgerrors.NewValidationError("invalid role")
	}
	return normalized, nil
}

func (s *Service) UpdateRole(ctx context.Context, actorID, workspaceID, targetID string, req dworkspace.UpdateMemberRoleRequest) (*dworkspace.WorkspaceMemberResponse, error) {
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
	if err := s.requireOwner(member.Role); err != nil {
		return nil, err
	}
	role := mworkspace.WorkspaceRole(req.Role)
	if role != mworkspace.RoleEditor && role != mworkspace.RoleMember {
		return nil, pkgerrors.NewValidationError("role must be editor or member")
	}
	target, err := s.workspaces.FindMember(ctx, workspaceID, targetID)
	if err != nil {
		return nil, err
	}
	if target.Role == mworkspace.RoleOwner {
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
	if err := s.requireOwner(member.Role); err != nil {
		return err
	}
	target, err := s.workspaces.FindMember(ctx, workspaceID, targetID)
	if err != nil {
		return err
	}
	if target.Role == mworkspace.RoleOwner {
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

func (s *Service) TransferOwner(ctx context.Context, actorID, workspaceID string, req dworkspace.TransferOwnerRequest) error {
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
	if err := s.requireOwner(member.Role); err != nil {
		return err
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
	s.broadcastUpdated(ctx, workspaceID, mworkspace.RoleEditor)
	return nil
}

func (s *Service) GetInvite(ctx context.Context, actorID, workspaceID string) (*dworkspace.WorkspaceInviteResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	member, err := s.memberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.requireEditor(member.Role); err != nil {
		return nil, err
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
	return &dworkspace.WorkspaceInviteResponse{InviteToken: &ws.InviteToken, ExpiresAt: ws.InviteExpiresAt}, nil
}

func (s *Service) DisableInvite(ctx context.Context, actorID, workspaceID string) error {
	if err := validate.UUID(workspaceID); err != nil {
		return err
	}
	member, err := s.memberOf(ctx, actorID, workspaceID)
	if err != nil {
		return err
	}
	if err := s.requireEditor(member.Role); err != nil {
		return err
	}
	if err := s.workspaces.UpdateInvite(ctx, workspaceID, "", nil); err != nil {
		return err
	}
	return nil
}

func (s *Service) JoinByInvite(ctx context.Context, actorID, token string) (*dworkspace.WorkspaceResponse, error) {
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
		if !common.IsNotFound(findErr) {
			return nil, findErr
		}
		member = nil
	}
	role := mworkspace.RoleMember
	added := false
	if member != nil {
		role = member.Role
	} else {
		if err := s.workspaces.AddMember(ctx, ws.ID, actorID, mworkspace.RoleMember); err != nil {
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

func (s *Service) memberOf(ctx context.Context, actorID, workspaceID string) (*mworkspace.WorkspaceMember, error) {
	return common.MemberOrForbidden(ctx, s.workspaces, workspaceID, actorID)
}

func (s *Service) requireOwner(role mworkspace.WorkspaceRole) error {
	return common.RequireOwner(role)
}

func (s *Service) requireEditor(role mworkspace.WorkspaceRole) error {
	return common.RequireEditor(role)
}

func (s *Service) bumpWorkspace(ctx context.Context, workspaceID string) {
	common.BumpWorkspace(ctx, s.cache, workspaceID)
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

func (s *Service) resolveOwner(ctx context.Context, workspaceID string) (*dworkspace.WorkspaceMemberResponse, error) {
	members, err := s.workspaces.ListMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	user, idx, err := s.findOwnerUser(ctx, members)
	if err != nil {
		return nil, err
	}
	if idx < 0 {
		return nil, nil
	}
	return memberToResponse(user, members[idx].Role), nil
}

func (s *Service) findOwnerUser(ctx context.Context, members []mworkspace.WorkspaceMember) (*muser.User, int, error) {
	for i := range members {
		if members[i].Role == mworkspace.RoleOwner {
			user, err := s.users.FindByID(ctx, members[i].UserID)
			if err != nil {
				if common.IsNotFound(err) {
					continue
				}
				return nil, -1, err
			}
			return user, i, nil
		}
	}
	return nil, -1, nil
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
		prefix = derivePrefixFromName(name)
	}
	if prefix == "" {
		prefix = "WS"
	}
	if err := validate.Prefix(prefix); err != nil {
		return "", err
	}
	return prefix, nil
}

func derivePrefixFromName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(name) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			if b.Len() == 3 {
				break
			}
		}
	}
	return b.String()
}

func workspaceToResponse(ws *mworkspace.Workspace, role mworkspace.WorkspaceRole, isFavorite bool) *dworkspace.WorkspaceResponse {
	return &dworkspace.WorkspaceResponse{
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

func (s *Service) broadcastUpdated(ctx context.Context, workspaceID string, role mworkspace.WorkspaceRole) {
	wsModel, err := s.workspaces.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return
	}
	s.hub.BroadcastToRoom(workspaceRoom(workspaceID), &ws.Message{Type: eventWorkspaceUpdated, Data: workspaceToResponse(wsModel, role, false)})
}

func workspaceRoom(workspaceID string) string {
	return "workspace:" + workspaceID
}

func memberToResponse(user *muser.User, role mworkspace.WorkspaceRole) *dworkspace.WorkspaceMemberResponse {
	return &dworkspace.WorkspaceMemberResponse{
		ID:          user.ID,
		Login:       user.Login,
		DisplayName: user.DisplayName,
		AvatarKey:   user.AvatarKey,
		Role:        string(role),
	}
}

var _ UseCase = (*Service)(nil)
