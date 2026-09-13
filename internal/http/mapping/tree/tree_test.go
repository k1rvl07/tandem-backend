package tree

import (
	"testing"
	"time"

	cboard "github.com/tandem/tandem/internal/contracts/board"
	ctask "github.com/tandem/tandem/internal/contracts/task"
	ctree "github.com/tandem/tandem/internal/contracts/tree"
	cworkspace "github.com/tandem/tandem/internal/contracts/workspace"
	dtotree "github.com/tandem/tandem/internal/http/dto/tree"
)

func TestToContractQuery(t *testing.T) {
	w := dtotree.TreeQuery{Tasks: "mine", Boards: "fav", Workspaces: "all"}
	got := ToContractQuery(w)
	if got.Tasks != "mine" || got.Boards != "fav" || got.Workspaces != "all" {
		t.Fatalf("ToContractQuery = %+v", got)
	}
}

func TestToWireWorkspaces(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	task := ctask.TaskResponse{ID: "t1", CreatedAt: at, UpdatedAt: at}
	in := []ctree.TreeWorkspaceResponse{
		{
			Workspace: cworkspace.WorkspaceResponse{ID: "ws1", CreatedAt: at, UpdatedAt: at},
			Boards: []ctree.TreeBoardResponse{
				{
					Board: cboard.BoardResponse{ID: "b1", CreatedAt: at, UpdatedAt: at},
					Tasks: []ctask.TaskResponse{task},
				},
			},
		},
	}
	got := ToWireWorkspaces(in)
	if len(got) != 1 || got[0].Workspace.ID != "ws1" {
		t.Fatalf("ToWireWorkspaces = %+v", got)
	}
	gotBoards := ToWireBoards(in[0].Boards)
	if len(gotBoards) != 1 || gotBoards[0].Board.ID != "b1" || len(gotBoards[0].Tasks) != 1 {
		t.Fatalf("ToWireBoards = %+v", gotBoards)
	}
}
