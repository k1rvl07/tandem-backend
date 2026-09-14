package treecontracts

import (
	boardcontracts "github.com/tandem/tandem/internal/contracts/board"
	taskcontracts "github.com/tandem/tandem/internal/contracts/task"
	workspacecontracts "github.com/tandem/tandem/internal/contracts/workspace"
)

type TreeBoardResponse struct {
	Board boardcontracts.BoardResponse `json:"board"`
	Tasks []taskcontracts.TaskResponse `json:"tasks"`
}

type TreeWorkspaceResponse struct {
	Workspace workspacecontracts.WorkspaceResponse `json:"workspace"`
	Boards    []TreeBoardResponse                  `json:"boards"`
}

type TreeQuery struct {
	Tasks      string
	Boards     string
	Workspaces string
}
