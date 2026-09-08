package tree

import (
	"github.com/tandem/tandem/internal/http/dto/board"
	"github.com/tandem/tandem/internal/http/dto/task"
	"github.com/tandem/tandem/internal/http/dto/workspace"
)

type TreeBoardResponse struct {
	Board board.BoardResponse `json:"board"`
	Tasks []task.TaskResponse `json:"tasks"`
}

type TreeWorkspaceResponse struct {
	Workspace workspace.WorkspaceResponse `json:"workspace"`
	Boards    []TreeBoardResponse         `json:"boards"`
}

type TreeQuery struct {
	Tasks      string `form:"tasks"`
	Boards     string `form:"boards"`
	Workspaces string `form:"workspaces"`
}
