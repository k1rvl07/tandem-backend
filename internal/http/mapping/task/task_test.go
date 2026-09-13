package task

import (
	"reflect"
	"testing"
	"time"

	ctask "github.com/tandem/tandem/internal/contracts/task"
	dtotask "github.com/tandem/tandem/internal/http/dto/task"
)

func TestToContractCreate(t *testing.T) {
	w := dtotask.CreateTaskRequest{
		ColumnID:    "c1",
		Title:       "Task",
		Description: "Desc",
		AssigneeID:  "u1",
		CuratorID:   "u2",
		ParentID:    "p1",
		DueDate:     "2026-06-01",
		IsUrgent:    true,
		IsHidden:    true,
		ImageKey:    "img/k",
	}
	got := ToContractCreate(w)
	want := ctask.CreateTaskRequest{
		ColumnID:    "c1",
		Title:       "Task",
		Description: "Desc",
		AssigneeID:  "u1",
		CuratorID:   "u2",
		ParentID:    "p1",
		DueDate:     "2026-06-01",
		IsUrgent:    true,
		IsHidden:    true,
		ImageKey:    "img/k",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ToContractCreate = %+v, want %+v", got, want)
	}
}

func TestToContractUpdate(t *testing.T) {
	title := "Task"
	due := "2026-06-01"
	position := 3
	urgent := true
	hidden := true
	imageKey := "img/k"
	w := dtotask.UpdateTaskRequest{
		BoardID:     strPtr("b1"),
		ColumnID:    strPtr("c1"),
		Title:       &title,
		Description: strPtr("Desc"),
		AssigneeID:  strPtr("u1"),
		CuratorID:   strPtr("u2"),
		ParentID:    strPtr("p1"),
		DueDate:     &due,
		Position:    &position,
		IsUrgent:    &urgent,
		IsHidden:    &hidden,
		ImageKey:    &imageKey,
	}
	got := ToContractUpdate(w)
	want := ctask.UpdateTaskRequest{
		BoardID:     strPtr("b1"),
		ColumnID:    strPtr("c1"),
		Title:       &title,
		Description: strPtr("Desc"),
		AssigneeID:  strPtr("u1"),
		CuratorID:   strPtr("u2"),
		ParentID:    strPtr("p1"),
		DueDate:     &due,
		Position:    &position,
		IsUrgent:    &urgent,
		IsHidden:    &hidden,
		ImageKey:    &imageKey,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ToContractUpdate = %+v, want %+v", got, want)
	}
}

func strPtr(s string) *string {
	return &s
}

func TestToContractListQuery(t *testing.T) {
	w := dtotask.ListWorkspaceTasksQuery{
		Q:               "q",
		BoardID:         "b1",
		AssigneeID:      "u1",
		Status:          "all",
		Only:            "subtasks",
		ExcludeSubtasks: true,
		Limit:           10,
		Offset:          5,
	}
	got := ToContractListQuery(w)
	if !reflect.DeepEqual(got, ctask.ListWorkspaceTasksQuery{
		Q:               "q",
		BoardID:         "b1",
		AssigneeID:      "u1",
		Status:          "all",
		Only:            "subtasks",
		ExcludeSubtasks: true,
		Limit:           10,
		Offset:          5,
	}) {
		t.Fatalf("ToContractListQuery = %+v", got)
	}
}

func TestToWireTask(t *testing.T) {
	due := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	user := &ctask.TaskUserResponse{
		ID:          "u1",
		Login:       "ivanov.ii",
		DisplayName: "Ivan",
		AvatarKey:   "k",
	}
	c := ctask.TaskResponse{
		ID:          "t1",
		DisplayID:   "WS-1",
		WorkspaceID: "ws1",
		BoardID:     "b1",
		BoardName:   "Board",
		ColumnID:    "c1",
		ColumnName:  "Backlog",
		Title:       "Task",
		Description: "Desc",
		Author:      user,
		Assignee:    nil,
		Curator:     user,
		ParentID:    "p1",
		DueDate:     &due,
		Position:    2,
		IsUrgent:    true,
		IsHidden:    true,
		ImageKey:    "img/k",
		CreatedAt:   due,
		UpdatedAt:   due,
	}
	got := ToWireTask(c)
	want := dtotask.TaskResponse{
		ID:          "t1",
		DisplayID:   "WS-1",
		WorkspaceID: "ws1",
		BoardID:     "b1",
		BoardName:   "Board",
		ColumnID:    "c1",
		ColumnName:  "Backlog",
		Title:       "Task",
		Description: "Desc",
		Author: &dtotask.TaskUserResponse{
			ID:          "u1",
			Login:       "ivanov.ii",
			DisplayName: "Ivan",
			AvatarKey:   "k",
		},
		Assignee:  nil,
		Curator:   &dtotask.TaskUserResponse{ID: "u1", Login: "ivanov.ii", DisplayName: "Ivan", AvatarKey: "k"},
		ParentID:  "p1",
		DueDate:   &due,
		Position:  2,
		IsUrgent:  true,
		IsHidden:  true,
		ImageKey:  "img/k",
		CreatedAt: due,
		UpdatedAt: due,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ToWireTask = %+v, want %+v", got, want)
	}
}

func TestToWireTasks(t *testing.T) {
	due := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	in := []ctask.TaskResponse{
		{ID: "t1", CreatedAt: due, UpdatedAt: due},
		{ID: "t2", CreatedAt: due, UpdatedAt: due},
	}
	got := ToWireTasks(in)
	if len(got) != 2 || got[0].ID != "t1" || got[1].ID != "t2" {
		t.Fatalf("ToWireTasks = %+v", got)
	}
}

func TestToWireDetail(t *testing.T) {
	due := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	parent := &ctask.TaskReference{
		ID: "t0", DisplayID: "WS-0", Title: "Parent", WorkspaceID: "ws1", BoardID: "b1",
		BoardName: "Board", ColumnID: "c1", ColumnName: "Backlog", IsUrgent: true, IsHidden: false,
	}
	sub := ctask.TaskReference{
		ID: "t2", DisplayID: "WS-2", Title: "Sub", WorkspaceID: "ws1", BoardID: "b1",
		BoardName: "Board", ColumnID: "c2", ColumnName: "Done", IsUrgent: false, IsHidden: true,
	}
	c := ctask.TaskDetailResponse{
		TaskResponse: ctask.TaskResponse{ID: "t1", CreatedAt: due, UpdatedAt: due},
		Parent:       parent,
		Subtasks:     []ctask.TaskReference{sub},
	}
	got := ToWireDetail(c)
	if got.ID != "t1" || got.Parent == nil || got.Parent.ID != "t0" || len(got.Subtasks) != 1 {
		t.Fatalf("ToWireDetail = %+v", got)
	}
	if got.Parent.WorkspaceID != "ws1" || got.Subtasks[0].ColumnName != "Done" {
		t.Fatalf("ToWireDetail nested fields mismatch: %+v", got)
	}
}

func TestToWireDetailNilParent(t *testing.T) {
	got := ToWireDetail(ctask.TaskDetailResponse{})
	if got.Parent != nil || len(got.Subtasks) != 0 {
		t.Fatalf("expected nil parent and no subtasks, got %+v", got)
	}
}

func TestToWireRefs(t *testing.T) {
	refs := []ctask.TaskReference{{ID: "t1", IsUrgent: true}, {ID: "t2", IsHidden: true}}
	got := ToWireRefs(refs)
	if len(got) != 2 || !got[0].IsUrgent || !got[1].IsHidden {
		t.Fatalf("ToWireRefs = %+v", got)
	}
}
