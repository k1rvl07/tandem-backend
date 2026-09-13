package mapping

import (
	mboard "github.com/tandem/tandem/internal/domain/models/board"
	mcolumn "github.com/tandem/tandem/internal/domain/models/column"
	mtask "github.com/tandem/tandem/internal/domain/models/task"
	dboard "github.com/tandem/tandem/internal/http/dto/board"
	dtask "github.com/tandem/tandem/internal/http/dto/task"
	"github.com/tandem/tandem/internal/usecase/shared/taskmap"
)

func BoardToResponse(board *mboard.Board) *dboard.BoardResponse {
	return &dboard.BoardResponse{
		ID:          board.ID,
		WorkspaceID: board.WorkspaceID,
		Name:        board.Name,
		Position:    board.Position,
		IsMain:      board.IsMain,
		CreatedAt:   board.CreatedAt,
		UpdatedAt:   board.UpdatedAt,
	}
}

func BuildColumnDetails(columns []*mcolumn.Column, tasks []*mtask.Task, users map[string]*dtask.TaskUserResponse, prefix, boardID, boardName string) []dboard.ColumnDetailResponse {
	columnDetails := make([]dboard.ColumnDetailResponse, 0, len(columns))
	for i := range columns {
		columnTasks := make([]dtask.TaskResponse, 0)
		for _, task := range tasks {
			if task.ColumnID != columns[i].ID {
				continue
			}
			if task.IsHidden {
				continue
			}
			columnTasks = append(columnTasks, taskToResponse(task, users, prefix, boardID, boardName, columns[i].Name))
		}
		columnDetails = append(columnDetails, dboard.ColumnDetailResponse{
			ID:        columns[i].ID,
			BoardID:   columns[i].BoardID,
			Name:      columns[i].Name,
			Position:  columns[i].Position,
			TaskCount: len(columnTasks),
			Tasks:     columnTasks,
			CreatedAt: columns[i].CreatedAt,
			UpdatedAt: columns[i].UpdatedAt,
		})
	}
	return columnDetails
}

func taskToResponse(task *mtask.Task, users map[string]*dtask.TaskUserResponse, prefix, boardID, boardName, columnName string) dtask.TaskResponse {
	return taskmap.BuildTaskResponse(task, users, taskmap.TaskResponseCtx{
		BoardID:    boardID,
		BoardName:  boardName,
		ColumnName: columnName,
		DisplayID:  taskmap.DisplayID(prefix, task.ID),
	})
}
