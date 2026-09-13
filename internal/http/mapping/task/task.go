package task

import (
	ctask "github.com/tandem/tandem/internal/contracts/task"
	dtotask "github.com/tandem/tandem/internal/http/dto/task"
)

func ToContractCreate(w dtotask.CreateTaskRequest) ctask.CreateTaskRequest {
	return ctask.CreateTaskRequest{
		ColumnID:    w.ColumnID,
		Title:       w.Title,
		Description: w.Description,
		AssigneeID:  w.AssigneeID,
		CuratorID:   w.CuratorID,
		ParentID:    w.ParentID,
		DueDate:     w.DueDate,
		IsUrgent:    w.IsUrgent,
		IsHidden:    w.IsHidden,
		ImageKey:    w.ImageKey,
	}
}

func ToContractUpdate(w dtotask.UpdateTaskRequest) ctask.UpdateTaskRequest {
	return ctask.UpdateTaskRequest{
		BoardID:     w.BoardID,
		ColumnID:    w.ColumnID,
		Title:       w.Title,
		Description: w.Description,
		AssigneeID:  w.AssigneeID,
		CuratorID:   w.CuratorID,
		ParentID:    w.ParentID,
		DueDate:     w.DueDate,
		Position:    w.Position,
		IsUrgent:    w.IsUrgent,
		IsHidden:    w.IsHidden,
		ImageKey:    w.ImageKey,
	}
}

func ToContractListQuery(w dtotask.ListWorkspaceTasksQuery) ctask.ListWorkspaceTasksQuery {
	return ctask.ListWorkspaceTasksQuery{
		Q:               w.Q,
		BoardID:         w.BoardID,
		AssigneeID:      w.AssigneeID,
		Status:          w.Status,
		Only:            w.Only,
		ExcludeSubtasks: w.ExcludeSubtasks,
		Limit:           w.Limit,
		Offset:          w.Offset,
	}
}

func ToWireRef(c ctask.TaskReference) dtotask.TaskReference {
	return dtotask.TaskReference{
		ID:          c.ID,
		DisplayID:   c.DisplayID,
		Title:       c.Title,
		WorkspaceID: c.WorkspaceID,
		BoardID:     c.BoardID,
		BoardName:   c.BoardName,
		ColumnID:    c.ColumnID,
		ColumnName:  c.ColumnName,
		IsUrgent:    c.IsUrgent,
		IsHidden:    c.IsHidden,
	}
}

func ToWireRefs(cc []ctask.TaskReference) []dtotask.TaskReference {
	out := make([]dtotask.TaskReference, 0, len(cc))
	for _, c := range cc {
		out = append(out, ToWireRef(c))
	}
	return out
}

func ToWireUser(c ctask.TaskUserResponse) dtotask.TaskUserResponse {
	return dtotask.TaskUserResponse{
		ID:          c.ID,
		Login:       c.Login,
		DisplayName: c.DisplayName,
		AvatarKey:   c.AvatarKey,
	}
}

func ToWireTask(c ctask.TaskResponse) dtotask.TaskResponse {
	return dtotask.TaskResponse{
		ID:          c.ID,
		DisplayID:   c.DisplayID,
		WorkspaceID: c.WorkspaceID,
		BoardID:     c.BoardID,
		BoardName:   c.BoardName,
		ColumnID:    c.ColumnID,
		ColumnName:  c.ColumnName,
		Title:       c.Title,
		Description: c.Description,
		Author:      wireTaskUser(c.Author),
		Assignee:    wireTaskUser(c.Assignee),
		Curator:     wireTaskUser(c.Curator),
		ParentID:    c.ParentID,
		DueDate:     c.DueDate,
		Position:    c.Position,
		IsUrgent:    c.IsUrgent,
		IsHidden:    c.IsHidden,
		ImageKey:    c.ImageKey,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

func wireTaskUser(c *ctask.TaskUserResponse) *dtotask.TaskUserResponse {
	if c == nil {
		return nil
	}
	u := ToWireUser(*c)
	return &u
}

func ToWireTasks(cc []ctask.TaskResponse) []dtotask.TaskResponse {
	out := make([]dtotask.TaskResponse, 0, len(cc))
	for _, c := range cc {
		out = append(out, ToWireTask(c))
	}
	return out
}

func ToWireDetail(c ctask.TaskDetailResponse) dtotask.TaskDetailResponse {
	var parent *dtotask.TaskReference
	if c.Parent != nil {
		p := ToWireRef(*c.Parent)
		parent = &p
	}
	return dtotask.TaskDetailResponse{
		TaskResponse: ToWireTask(c.TaskResponse),
		Parent:       parent,
		Subtasks:     ToWireRefs(c.Subtasks),
	}
}
