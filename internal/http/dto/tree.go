package dto

type TreeBoardResponse struct {
	Board BoardResponse  `json:"board"`
	Tasks []TaskResponse `json:"tasks"`
}

type TreeWorkspaceResponse struct {
	Workspace WorkspaceResponse   `json:"workspace"`
	Boards    []TreeBoardResponse `json:"boards"`
}

type TreeQuery struct {
	Tasks      string `form:"tasks"`
	Boards     string `form:"boards"`
	Workspaces string `form:"workspaces"`
}
