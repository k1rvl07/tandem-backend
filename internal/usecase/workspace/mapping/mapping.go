package mapping

import (
	muser "github.com/tandem/tandem/internal/domain/models/user"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	dworkspace "github.com/tandem/tandem/internal/http/dto/workspace"
)

func WorkspaceResponse(ws *mworkspace.Workspace, role mworkspace.WorkspaceRole, isFavorite bool) *dworkspace.WorkspaceResponse {
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

func MemberResponse(user *muser.User, role mworkspace.WorkspaceRole) *dworkspace.WorkspaceMemberResponse {
	return &dworkspace.WorkspaceMemberResponse{
		ID:          user.ID,
		Login:       user.Login,
		DisplayName: user.DisplayName,
		AvatarKey:   user.AvatarKey,
		Role:        string(role),
	}
}
