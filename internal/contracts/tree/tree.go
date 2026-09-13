package treecontracts

import (
	boardcontracts "github.com/tandem/tandem/internal/contracts/board"
	taskcontracts "github.com/tandem/tandem/internal/contracts/task"
	workspacecontracts "github.com/tandem/tandem/internal/contracts/workspace"
)

type TreeBoardResponse struct {
	Board boardcontracts.BoardResponse
	Tasks []taskcontracts.TaskResponse
}

type TreeWorkspaceResponse struct {
	Workspace workspacecontracts.WorkspaceResponse
	Boards    []TreeBoardResponse
}

type TreeQuery struct {
	Tasks      string
	Boards     string
	Workspaces string
}
