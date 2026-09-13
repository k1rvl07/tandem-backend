package board

import (
	cboard "github.com/tandem/tandem/internal/contracts/board"
	dtoboard "github.com/tandem/tandem/internal/http/dto/board"
	mptask "github.com/tandem/tandem/internal/http/mapping/task"
)

func ToContractCreate(w dtoboard.CreateBoardRequest) cboard.CreateBoardRequest {
	return cboard.CreateBoardRequest{Name: w.Name}
}

func ToContractUpdate(w dtoboard.UpdateBoardRequest) cboard.UpdateBoardRequest {
	return cboard.UpdateBoardRequest{Name: w.Name}
}

func ToContractReorder(w dtoboard.ReorderBoardsRequest) cboard.ReorderBoardsRequest {
	return cboard.ReorderBoardsRequest{BoardIDs: w.BoardIDs}
}

func ToWireBoard(c cboard.BoardResponse) dtoboard.BoardResponse {
	return dtoboard.BoardResponse{
		ID:          c.ID,
		WorkspaceID: c.WorkspaceID,
		Name:        c.Name,
		Position:    c.Position,
		IsMain:      c.IsMain,
		IsFavorite:  c.IsFavorite,
		TaskCount:   c.TaskCount,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

func ToWireBoards(cc []cboard.BoardResponse) []dtoboard.BoardResponse {
	out := make([]dtoboard.BoardResponse, 0, len(cc))
	for _, c := range cc {
		out = append(out, ToWireBoard(c))
	}
	return out
}

func ToWireDetail(c cboard.BoardDetailResponse) dtoboard.BoardDetailResponse {
	columns := make([]dtoboard.ColumnDetailResponse, 0, len(c.Columns))
	for _, col := range c.Columns {
		columns = append(columns, dtoboard.ColumnDetailResponse{
			ID:        col.ID,
			BoardID:   col.BoardID,
			Name:      col.Name,
			Position:  col.Position,
			TaskCount: col.TaskCount,
			Tasks:     mptask.ToWireTasks(col.Tasks),
			CreatedAt: col.CreatedAt,
			UpdatedAt: col.UpdatedAt,
		})
	}
	return dtoboard.BoardDetailResponse{
		ID:          c.ID,
		WorkspaceID: c.WorkspaceID,
		Name:        c.Name,
		Position:    c.Position,
		IsMain:      c.IsMain,
		Columns:     columns,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}
