package workspace

import (
	cworkspace "github.com/tandem/tandem/internal/contracts/workspace"
	dtoworkspace "github.com/tandem/tandem/internal/http/dto/workspace"
)

func ToContractCreate(w dtoworkspace.CreateWorkspaceRequest) cworkspace.CreateWorkspaceRequest {
	return cworkspace.CreateWorkspaceRequest{
		Name:        w.Name,
		Description: w.Description,
		Prefix:      w.Prefix,
	}
}

func ToContractUpdate(w dtoworkspace.UpdateWorkspaceRequest) cworkspace.UpdateWorkspaceRequest {
	return cworkspace.UpdateWorkspaceRequest{
		Name:        w.Name,
		Description: w.Description,
		Prefix:      w.Prefix,
	}
}

func ToContractAddMember(w dtoworkspace.AddMemberRequest) cworkspace.AddMemberRequest {
	return cworkspace.AddMemberRequest{
		Login: w.Login,
		Role:  w.Role,
	}
}

func ToContractUpdateMemberRole(w dtoworkspace.UpdateMemberRoleRequest) cworkspace.UpdateMemberRoleRequest {
	return cworkspace.UpdateMemberRoleRequest{Role: w.Role}
}

func ToContractTransferOwner(w dtoworkspace.TransferOwnerRequest) cworkspace.TransferOwnerRequest {
	return cworkspace.TransferOwnerRequest{UserID: w.UserID}
}

func ToWireMember(c cworkspace.WorkspaceMemberResponse) dtoworkspace.WorkspaceMemberResponse {
	return dtoworkspace.WorkspaceMemberResponse{
		ID:          c.ID,
		Login:       c.Login,
		DisplayName: c.DisplayName,
		AvatarKey:   c.AvatarKey,
		Role:        c.Role,
		JoinedAt:    c.JoinedAt,
	}
}

func ToWireMembers(cc []cworkspace.WorkspaceMemberResponse) []dtoworkspace.WorkspaceMemberResponse {
	out := make([]dtoworkspace.WorkspaceMemberResponse, 0, len(cc))
	for _, c := range cc {
		out = append(out, ToWireMember(c))
	}
	return out
}

func ToWireWorkspace(c cworkspace.WorkspaceResponse) dtoworkspace.WorkspaceResponse {
	var owner *dtoworkspace.WorkspaceMemberResponse
	if c.Owner != nil {
		o := ToWireMember(*c.Owner)
		owner = &o
	}
	return dtoworkspace.WorkspaceResponse{
		ID:          c.ID,
		Name:        c.Name,
		Description: c.Description,
		Prefix:      c.Prefix,
		Theme:       c.Theme,
		Role:        c.Role,
		IsFavorite:  c.IsFavorite,
		Owner:       owner,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

func ToWireWorkspaces(cc []cworkspace.WorkspaceResponse) []dtoworkspace.WorkspaceResponse {
	out := make([]dtoworkspace.WorkspaceResponse, 0, len(cc))
	for _, c := range cc {
		out = append(out, ToWireWorkspace(c))
	}
	return out
}

func ToWireDetail(c cworkspace.WorkspaceDetailResponse) dtoworkspace.WorkspaceDetailResponse {
	return dtoworkspace.WorkspaceDetailResponse{
		WorkspaceResponse: ToWireWorkspace(c.WorkspaceResponse),
		Members:           ToWireMembers(c.Members),
	}
}

func ToWireInvite(c cworkspace.WorkspaceInviteResponse) dtoworkspace.WorkspaceInviteResponse {
	return dtoworkspace.WorkspaceInviteResponse{
		InviteToken: c.InviteToken,
		ExpiresAt:   c.ExpiresAt,
	}
}
