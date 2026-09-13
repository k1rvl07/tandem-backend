package crud

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	mboard "github.com/tandem/tandem/internal/domain/models/board"
	mcolumn "github.com/tandem/tandem/internal/domain/models/column"
	mfavorite "github.com/tandem/tandem/internal/domain/models/favorite"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	dworkspace "github.com/tandem/tandem/internal/http/dto/workspace"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	cacheutil "github.com/tandem/tandem/internal/usecase/shared/cache"
	"github.com/tandem/tandem/internal/usecase/workspace/core"
)

const maxDescriptionLen = 400

type Crud struct {
	*core.Core
}

func New(c *core.Core) *Crud {
	return &Crud{Core: c}
}

func (s *Crud) Create(ctx context.Context, actorID string, req dworkspace.CreateWorkspaceRequest) (*dworkspace.WorkspaceResponse, error) {
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
	if err := s.Workspaces.CreateWorkspace(ctx, ws); err != nil {
		return nil, err
	}
	if err := s.Workspaces.AddMember(ctx, ws.ID, actorID, mworkspace.RoleOwner); err != nil {
		_ = s.Workspaces.DeleteWorkspace(ctx, ws.ID)
		return nil, err
	}
	board, err := s.createMainBoard(ctx, ws.ID)
	if err != nil {
		if board != nil {
			_ = s.Boards.DeleteBoard(ctx, board.ID)
		}
		_ = s.Workspaces.DeleteWorkspace(ctx, ws.ID)
		return nil, err
	}
	ownerUser, err := s.Users.FindByID(ctx, actorID)
	if err != nil {
		return nil, err
	}
	resp := s.WorkspaceResponse(ws, mworkspace.RoleOwner, false)
	resp.Owner = s.MemberResponse(ownerUser, mworkspace.RoleOwner)
	cacheutil.Bump(ctx, s.Cache, cacheutil.UVerKey+actorID)
	return resp, nil
}

func (s *Crud) createMainBoard(ctx context.Context, workspaceID string) (*mboard.Board, error) {
	board := &mboard.Board{
		ID:          uuid.New().String(),
		WorkspaceID: workspaceID,
		Name:        "Main",
		Position:    0,
		IsMain:      true,
	}
	if err := s.Boards.CreateBoard(ctx, board); err != nil {
		return nil, err
	}
	for i, name := range mcolumn.DefaultColumnNames {
		column := &mcolumn.Column{
			ID:       uuid.New().String(),
			BoardID:  board.ID,
			Name:     name,
			Position: i,
		}
		if err := s.Columns.CreateColumn(ctx, column); err != nil {
			return board, err
		}
	}
	return board, nil
}

func (s *Crud) List(ctx context.Context, actorID string) ([]dworkspace.WorkspaceResponse, error) {
	uver := cacheutil.Version(ctx, s.Cache, cacheutil.UVerKey+actorID)
	listKey := fmt.Sprintf("u:%s:t:v1:wslist:%s", actorID, uver)
	var cached []dworkspace.WorkspaceResponse
	if cacheutil.Load(ctx, s.Cache, listKey, &cached) {
		return cached, nil
	}
	memberships, err := s.Workspaces.ListWorkspacesForUser(ctx, actorID)
	if err != nil {
		return nil, err
	}
	favorites, err := s.Favorites.ListFavoriteTargets(ctx, actorID, mfavorite.FavoriteWorkspace)
	if err != nil {
		return nil, err
	}
	responses := make([]dworkspace.WorkspaceResponse, 0, len(memberships))
	for i := range memberships {
		resp := s.WorkspaceResponse(&memberships[i].Workspace, memberships[i].Role, favorites[memberships[i].Workspace.ID])
		owner, err := s.ResolveOwner(ctx, memberships[i].Workspace.ID)
		if err != nil {
			return nil, err
		}
		resp.Owner = owner
		responses = append(responses, *resp)
	}
	cacheutil.Store(ctx, s.Cache, listKey, responses, cacheutil.TTL)
	return responses, nil
}

func (s *Crud) Get(ctx context.Context, actorID, workspaceID string) (*dworkspace.WorkspaceDetailResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	wsver := cacheutil.Version(ctx, s.Cache, cacheutil.WSVerKey+workspaceID)
	uver := cacheutil.Version(ctx, s.Cache, cacheutil.UVerKey+actorID)
	detailKey := fmt.Sprintf("u:%s:t:v1:ws:%s:%s:%s", actorID, workspaceID, wsver, uver)
	var cached dworkspace.WorkspaceDetailResponse
	if cacheutil.Load(ctx, s.Cache, detailKey, &cached) {
		return &cached, nil
	}
	ws, err := s.Workspaces.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	member, err := s.MemberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	isFavorite, err := s.Favorites.IsFavorite(ctx, actorID, mfavorite.FavoriteWorkspace, workspaceID)
	if err != nil {
		return nil, err
	}
	members, err := s.Workspaces.ListMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	memberResponses, err := s.BuildMemberResponses(ctx, members)
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
	cacheutil.Store(ctx, s.Cache, detailKey, detail, cacheutil.TTL)
	return detail, nil
}

func (s *Crud) Update(ctx context.Context, actorID, workspaceID string, req dworkspace.UpdateWorkspaceRequest) (*dworkspace.WorkspaceResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	member, err := s.MemberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.RequireEditor(member.Role); err != nil {
		return nil, err
	}
	if err := validateWorkspacePayload(req.Name, req.Description); err != nil {
		return nil, err
	}
	prefix, err := normalizePrefix(req.Prefix, req.Name)
	if err != nil {
		return nil, err
	}
	wsModel, err := s.Workspaces.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	wsModel.Name = strings.TrimSpace(req.Name)
	wsModel.Description = strings.TrimSpace(req.Description)
	wsModel.Prefix = prefix
	if err := s.Workspaces.UpdateWorkspace(ctx, wsModel); err != nil {
		return nil, err
	}
	response := s.WorkspaceResponse(wsModel, member.Role, false)
	s.BumpWorkspace(ctx, workspaceID)
	s.BumpMembers(ctx, workspaceID)
	s.Hub.BroadcastToRoom(s.Room(workspaceID), &ws.Message{Type: core.EventWorkspaceUpdated, Data: response})
	return response, nil
}

func (s *Crud) SetTheme(ctx context.Context, actorID, workspaceID, theme string) (*dworkspace.WorkspaceResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	theme = strings.TrimSpace(theme)
	if err := validate.Color(theme); err != nil {
		return nil, err
	}
	member, err := s.MemberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.RequireEditor(member.Role); err != nil {
		return nil, err
	}
	wsModel, err := s.Workspaces.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	wsModel.Theme = theme
	if err := s.Workspaces.UpdateWorkspace(ctx, wsModel); err != nil {
		return nil, err
	}
	response := s.WorkspaceResponse(wsModel, member.Role, false)
	s.BumpWorkspace(ctx, workspaceID)
	s.BumpMembers(ctx, workspaceID)
	s.Hub.BroadcastToRoom(s.Room(workspaceID), &ws.Message{Type: core.EventWorkspaceUpdated, Data: response})
	return response, nil
}

func (s *Crud) Delete(ctx context.Context, actorID, workspaceID string) error {
	if err := validate.UUID(workspaceID); err != nil {
		return err
	}
	member, err := s.MemberOf(ctx, actorID, workspaceID)
	if err != nil {
		return err
	}
	if err := s.RequireOwner(member.Role); err != nil {
		return err
	}
	s.BumpMembers(ctx, workspaceID)
	keys, err := s.Tasks.CollectWorkspaceKeys(ctx, workspaceID)
	if err != nil {
		return err
	}
	s.Files.RemoveMany(ctx, keys)
	if err := s.Workspaces.DeleteWorkspace(ctx, workspaceID); err != nil {
		return err
	}
	s.BumpWorkspace(ctx, workspaceID)
	s.Hub.BroadcastToRoom(s.Room(workspaceID), &ws.Message{
		Type: core.EventMemberKicked,
		Data: map[string]string{"workspace_id": workspaceID},
	})
	return nil
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
