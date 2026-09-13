package access

import (
	"context"
	"errors"
	"testing"

	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/usecase/testutil"
)

func TestMemberOrForbidden(t *testing.T) {
	repo := testutil.NewFakeWorkspaceRepo()
	repo.AddMemberFixture("ws1", "u1", mworkspace.RoleOwner)
	ctx := context.Background()

	member, err := MemberOrForbidden(ctx, repo, "ws1", "u1")
	if err != nil {
		t.Fatalf("MemberOrForbidden = %v", err)
	}
	if member == nil || member.Role != mworkspace.RoleOwner {
		t.Fatalf("member = %+v", member)
	}

	_, err = MemberOrForbidden(ctx, repo, "ws1", "nobody")
	if !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for non-member, got %v", err)
	}
}

func TestBoardInWorkspace(t *testing.T) {
	boards := testutil.NewFakeBoardRepo()
	boards.AddBoardFixture("b1", "ws1", "Board")
	ctx := context.Background()

	b, err := BoardInWorkspace(ctx, boards, "ws1", "b1")
	if err != nil || b.ID != "b1" {
		t.Fatalf("BoardInWorkspace = %v, %v", b, err)
	}

	_, err = BoardInWorkspace(ctx, boards, "ws2", "b1")
	if !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-workspace board, got %v", err)
	}

	_, err = BoardInWorkspace(ctx, boards, "ws1", "missing")
	if !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing board, got %v", err)
	}
}

func TestTaskInWorkspace(t *testing.T) {
	boards := testutil.NewFakeBoardRepo()
	columns := testutil.NewFakeColumnRepo()
	tasks := testutil.NewFakeTaskRepo()
	ctx := context.Background()

	boards.AddBoardFixture("b1", "ws1", "Board")
	columns.AddColumnFixture("c1", "b1", "Backlog", 0)
	tasks.AddTaskFixture("t1", "c1", "Task", 0)

	task, err := TaskInWorkspace(ctx, tasks, columns, boards, "ws1", "t1")
	if err != nil || task.ID != "t1" {
		t.Fatalf("TaskInWorkspace = %v, %v", task, err)
	}

	_, err = TaskInWorkspace(ctx, tasks, columns, boards, "ws2", "t1")
	if !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for cross-workspace task, got %v", err)
	}

	_, err = TaskInWorkspace(ctx, tasks, columns, boards, "ws1", "missing")
	if !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing task, got %v", err)
	}
}

func TestRequireRoles(t *testing.T) {
	if err := RequireEditor(mworkspace.RoleEditor); err != nil {
		t.Fatalf("editor should edit: %v", err)
	}
	if err := RequireEditor(mworkspace.RoleOwner); err != nil {
		t.Fatalf("owner should edit: %v", err)
	}
	if err := RequireEditor(mworkspace.RoleMember); !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("member editing should be forbidden, got %v", err)
	}
	if err := RequireOwner(mworkspace.RoleMember); !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("member owning should be forbidden, got %v", err)
	}
	if err := RequireOwner(mworkspace.RoleOwner); err != nil {
		t.Fatalf("owner should own: %v", err)
	}
}
