package board

import (
	"reflect"
	"testing"
	"time"

	cboard "github.com/tandem/tandem/internal/contracts/board"
	ctask "github.com/tandem/tandem/internal/contracts/task"
	dtoboard "github.com/tandem/tandem/internal/http/dto/board"
)

func TestToContractCreate(t *testing.T) {
	got := ToContractCreate(dtoboard.CreateBoardRequest{Name: "Board"})
	if !reflect.DeepEqual(got, cboard.CreateBoardRequest{Name: "Board"}) {
		t.Fatalf("ToContractCreate = %+v", got)
	}
}

func TestToContractUpdate(t *testing.T) {
	got := ToContractUpdate(dtoboard.UpdateBoardRequest{Name: "Renamed"})
	if !reflect.DeepEqual(got, cboard.UpdateBoardRequest{Name: "Renamed"}) {
		t.Fatalf("ToContractUpdate = %+v", got)
	}
}

func TestToContractReorder(t *testing.T) {
	got := ToContractReorder(dtoboard.ReorderBoardsRequest{BoardIDs: []string{"b2", "b1"}})
	if !reflect.DeepEqual(got, cboard.ReorderBoardsRequest{BoardIDs: []string{"b2", "b1"}}) {
		t.Fatalf("ToContractReorder = %+v", got)
	}
}

func TestToWireBoard(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := cboard.BoardResponse{
		ID: "b1", WorkspaceID: "ws1", Name: "Board", Position: 2, IsMain: true,
		IsFavorite: true, TaskCount: 5, CreatedAt: at, UpdatedAt: at,
	}
	got := ToWireBoard(c)
	if got.ID != "b1" || !got.IsMain || !got.IsFavorite || got.TaskCount != 5 {
		t.Fatalf("ToWireBoard = %+v", got)
	}
}

func TestToWireBoards(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got := ToWireBoards([]cboard.BoardResponse{{ID: "b1", CreatedAt: at}, {ID: "b2", CreatedAt: at}})
	if len(got) != 2 {
		t.Fatalf("ToWireBoards len = %d", len(got))
	}
}

func TestToWireDetail(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	task := ctask.TaskResponse{ID: "t1", CreatedAt: at, UpdatedAt: at}
	c := cboard.BoardDetailResponse{
		ID: "b1", WorkspaceID: "ws1", Name: "Board", Position: 0, IsMain: false, CreatedAt: at, UpdatedAt: at,
		Columns: []cboard.ColumnDetailResponse{
			{
				ID: "c1", BoardID: "b1", Name: "Backlog", Position: 0, TaskCount: 1,
				Tasks: []ctask.TaskResponse{task}, CreatedAt: at, UpdatedAt: at,
			},
		},
	}
	got := ToWireDetail(c)
	if got.ID != "b1" || len(got.Columns) != 1 || got.Columns[0].TaskCount != 1 {
		t.Fatalf("ToWireDetail = %+v", got)
	}
	if len(got.Columns[0].Tasks) != 1 || got.Columns[0].Tasks[0].ID != "t1" {
		t.Fatalf("ToWireDetail nested tasks mismatch: %+v", got.Columns[0].Tasks)
	}
}
