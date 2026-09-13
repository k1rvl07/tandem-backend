package workspace

import (
	"reflect"
	"testing"
	"time"

	cworkspace "github.com/tandem/tandem/internal/contracts/workspace"
	dtoworkspace "github.com/tandem/tandem/internal/http/dto/workspace"
)

func TestToContractCreate(t *testing.T) {
	got := ToContractCreate(dtoworkspace.CreateWorkspaceRequest{
		Name:        "Team",
		Description: "Desc",
		Prefix:      "T",
	})
	want := cworkspace.CreateWorkspaceRequest{Name: "Team", Description: "Desc", Prefix: "T"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ToContractCreate = %+v", got)
	}
}

func TestToContractUpdate(t *testing.T) {
	got := ToContractUpdate(dtoworkspace.UpdateWorkspaceRequest{
		Name:        "Team",
		Description: "Desc",
		Prefix:      "T",
	})
	if !reflect.DeepEqual(got, cworkspace.UpdateWorkspaceRequest{Name: "Team", Description: "Desc", Prefix: "T"}) {
		t.Fatalf("ToContractUpdate = %+v", got)
	}
}

func TestToContractMemberOps(t *testing.T) {
	got := ToContractAddMember(dtoworkspace.AddMemberRequest{Login: "u", Role: "editor"})
	if !reflect.DeepEqual(got, cworkspace.AddMemberRequest{Login: "u", Role: "editor"}) {
		t.Fatalf("ToContractAddMember = %+v", got)
	}
	gotRole := ToContractUpdateMemberRole(dtoworkspace.UpdateMemberRoleRequest{Role: "member"})
	if !reflect.DeepEqual(gotRole, cworkspace.UpdateMemberRoleRequest{Role: "member"}) {
		t.Fatalf("ToContractUpdateMemberRole = %+v", gotRole)
	}
	gotTransfer := ToContractTransferOwner(dtoworkspace.TransferOwnerRequest{UserID: "u1"})
	if !reflect.DeepEqual(gotTransfer, cworkspace.TransferOwnerRequest{UserID: "u1"}) {
		t.Fatalf("ToContractTransferOwner = %+v", gotTransfer)
	}
}

func TestToWireWorkspace(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	owner := &cworkspace.WorkspaceMemberResponse{
		ID: "u1", Login: "admin", DisplayName: "Admin", AvatarKey: "k", Role: "owner", JoinedAt: at,
	}
	c := cworkspace.WorkspaceResponse{
		ID: "ws1", Name: "Team", Description: "Desc", Prefix: "T", Theme: "1d4ed8",
		Role: "member", IsFavorite: true, Owner: owner, CreatedAt: at, UpdatedAt: at,
	}
	got := ToWireWorkspace(c)
	if got.ID != "ws1" || got.Owner == nil || got.Owner.Role != "owner" || got.Owner.JoinedAt != at {
		t.Fatalf("ToWireWorkspace = %+v", got)
	}
}

func TestToWireWorkspaceNilOwner(t *testing.T) {
	got := ToWireWorkspace(cworkspace.WorkspaceResponse{})
	if got.Owner != nil {
		t.Fatalf("expected nil owner, got %+v", got.Owner)
	}
}

func TestToWireMembers(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	in := []cworkspace.WorkspaceMemberResponse{
		{ID: "u1", Login: "a", Role: "owner", JoinedAt: at},
		{ID: "u2", Login: "b", Role: "member", JoinedAt: at},
	}
	got := ToWireMembers(in)
	if len(got) != 2 || got[0].Role != "owner" || got[1].Login != "b" {
		t.Fatalf("ToWireMembers = %+v", got)
	}
}

func TestToWireWorkspaces(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got := ToWireWorkspaces([]cworkspace.WorkspaceResponse{{ID: "ws1", CreatedAt: at}, {ID: "ws2", CreatedAt: at}})
	if len(got) != 2 {
		t.Fatalf("ToWireWorkspaces len = %d", len(got))
	}
}

func TestToWireDetail(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := cworkspace.WorkspaceDetailResponse{
		WorkspaceResponse: cworkspace.WorkspaceResponse{ID: "ws1", CreatedAt: at, UpdatedAt: at},
		Members:           []cworkspace.WorkspaceMemberResponse{{ID: "u1", Role: "owner", JoinedAt: at}},
	}
	got := ToWireDetail(c)
	if got.ID != "ws1" || len(got.Members) != 1 {
		t.Fatalf("ToWireDetail = %+v", got)
	}
}

func TestToWireInvite(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	token := "tok"
	c := cworkspace.WorkspaceInviteResponse{InviteToken: &token, ExpiresAt: &at}
	got := ToWireInvite(c)
	if got.InviteToken == nil || *got.InviteToken != "tok" || got.ExpiresAt == nil {
		t.Fatalf("ToWireInvite = %+v", got)
	}
}
