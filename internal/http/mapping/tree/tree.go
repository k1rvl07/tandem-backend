package tree

import (
	ctree "github.com/tandem/tandem/internal/contracts/tree"
	dtotree "github.com/tandem/tandem/internal/http/dto/tree"
	mpboard "github.com/tandem/tandem/internal/http/mapping/board"
	mptask "github.com/tandem/tandem/internal/http/mapping/task"
	mpworkspace "github.com/tandem/tandem/internal/http/mapping/workspace"
)

func ToContractQuery(w dtotree.TreeQuery) ctree.TreeQuery {
	return ctree.TreeQuery{
		Tasks:      w.Tasks,
		Boards:     w.Boards,
		Workspaces: w.Workspaces,
	}
}

func ToWireWorkspaces(cc []ctree.TreeWorkspaceResponse) []dtotree.TreeWorkspaceResponse {
	out := make([]dtotree.TreeWorkspaceResponse, 0, len(cc))
	for _, c := range cc {
		out = append(out, dtotree.TreeWorkspaceResponse{
			Workspace: mpworkspace.ToWireWorkspace(c.Workspace),
			Boards:    ToWireBoards(c.Boards),
		})
	}
	return out
}

func ToWireBoards(bb []ctree.TreeBoardResponse) []dtotree.TreeBoardResponse {
	out := make([]dtotree.TreeBoardResponse, 0, len(bb))
	for _, b := range bb {
		out = append(out, dtotree.TreeBoardResponse{
			Board: mpboard.ToWireBoard(b.Board),
			Tasks: mptask.ToWireTasks(b.Tasks),
		})
	}
	return out
}
