package dto

type CreateTaskRequest struct {
	ColumnID    string `json:"column_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	AssigneeID  string `json:"assignee_id"`
	Priority    string `json:"priority"`
	DueDate     string `json:"due_date"`
}

type UpdateTaskRequest struct {
	ColumnID    *string `json:"column_id"`
	Title       *string `json:"title"`
	Description *string `json:"description"`
	AssigneeID  *string `json:"assignee_id"`
	Priority    *string `json:"priority"`
	DueDate     *string `json:"due_date"`
	Position    *int    `json:"position"`
}
