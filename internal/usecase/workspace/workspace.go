package workspace

import (
	"context"

	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	dworkspace "github.com/tandem/tandem/internal/http/dto/workspace"
	file "github.com/tandem/tandem/internal/usecase/file"
	"github.com/tandem/tandem/internal/usecase/workspace/core"
	"github.com/tandem/tandem/internal/usecase/workspace/crud"
	"github.com/tandem/tandem/internal/usecase/workspace/invites"
	"github.com/tandem/tandem/internal/usecase/workspace/members"
	"go.uber.org/zap"
)

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

type Service struct {
	*core.Core
	*crud.Crud
	*members.Members
	*invites.Invites
}

func NewService(deps Deps) *Service {
	c := core.New(deps.Workspaces, deps.Users, deps.Favorites, deps.Boards, deps.Columns, deps.Tasks, deps.Files, deps.Hub, deps.Cache, deps.Logger)
	return &Service{
		Core:    c,
		Crud:    crud.New(c),
		Members: members.New(c),
		Invites: invites.New(c),
	}
}

var _ UseCase = (*Service)(nil)
