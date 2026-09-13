package workspace

import (
	"context"

	dworkspace "github.com/tandem/tandem/internal/contracts/workspace"
	"github.com/tandem/tandem/internal/usecase/shared/base"
	"github.com/tandem/tandem/internal/usecase/workspace/core"
	"github.com/tandem/tandem/internal/usecase/workspace/features/crud"
	"github.com/tandem/tandem/internal/usecase/workspace/features/invites"
	"github.com/tandem/tandem/internal/usecase/workspace/features/members"
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
	base.Deps
}

type Service struct {
	*core.Core
	*crud.Crud
	*members.Members
	*invites.Invites
}

func NewService(deps Deps) *Service {
	c := core.New(deps.Deps)
	return &Service{
		Core:    c,
		Crud:    crud.New(c),
		Members: members.New(c),
		Invites: invites.New(c),
	}
}

var _ UseCase = (*Service)(nil)
