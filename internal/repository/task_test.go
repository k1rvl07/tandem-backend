package repository

import (
	"testing"
	"time"

	"github.com/tandem/tandem/internal/domain/models"
)

func TestTaskRepoCRUD(t *testing.T) {
	db := newTestDB(t)
	taskRepo := NewTaskRepo(db)
	columnRepo := NewColumnRepo(db)
	boardRepo := NewBoardRepo(db)

	ws := newWorkspace(newID(), "Team")
	must(t, NewWorkspaceRepo(db).CreateWorkspace(testCtx, ws))
	board := newBoard(ws.ID, "Board", 0, true)
	must(t, boardRepo.CreateBoard(testCtx, board))
	column := &models.Column{ID: newID(), BoardID: board.ID, Name: "Backlog", Position: 0}
	must(t, columnRepo.CreateColumn(testCtx, column))

	due := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	tk := &models.Task{ID: newID(), ColumnID: column.ID, Title: "Task", Description: "desc", Position: 0, IsUrgent: true, DueDate: &due}
	must(t, taskRepo.CreateTask(testCtx, tk))
	if tk.CreatedAt.IsZero() {
		t.Fatalf("created_at not set")
	}

	got, err := taskRepo.FindTaskByID(testCtx, tk.ID)
	must(t, err)
	if got.Title != "Task" || !got.IsUrgent || got.DueDate == nil || got.DueDate.Sub(due) > time.Second {
		t.Fatalf("unexpected task: %+v", got)
	}
	mustNotFound(t, errOf(taskRepo.FindTaskByID(testCtx, newID())))

	tk.Title = "Updated"
	tk.Position = 2
	tk.DueDate = nil
	tk.IsUrgent = false
	must(t, taskRepo.UpdateTask(testCtx, tk))
	got, err = taskRepo.FindTaskByID(testCtx, tk.ID)
	must(t, err)
	if got.Title != "Updated" || got.DueDate != nil || got.IsUrgent {
		t.Fatalf("update not applied: %+v", got)
	}
}

func TestTaskRepoLists(t *testing.T) {
	db := newTestDB(t)
	taskRepo := NewTaskRepo(db)
	columnRepo := NewColumnRepo(db)
	boardRepo := NewBoardRepo(db)

	ws := newWorkspace(newID(), "Team")
	must(t, NewWorkspaceRepo(db).CreateWorkspace(testCtx, ws))
	boardA := newBoard(ws.ID, "A", 0, true)
	boardB := newBoard(ws.ID, "B", 1, false)
	must(t, boardRepo.CreateBoard(testCtx, boardA))
	must(t, boardRepo.CreateBoard(testCtx, boardB))
	colA := &models.Column{ID: newID(), BoardID: boardA.ID, Name: "Backlog"}
	colB := &models.Column{ID: newID(), BoardID: boardB.ID, Name: "Done"}
	must(t, columnRepo.CreateColumn(testCtx, colA))
	must(t, columnRepo.CreateColumn(testCtx, colB))

	for i, col := range []*models.Column{colA, colB} {
		task := &models.Task{ID: newID(), ColumnID: col.ID, Title: "T", Position: i}
		must(t, taskRepo.CreateTask(testCtx, task))
	}

	byBoard, err := taskRepo.ListTasksForBoard(testCtx, boardA.ID)
	must(t, err)
	if len(byBoard) != 1 || byBoard[0].ColumnID != colA.ID {
		t.Fatalf("unexpected board tasks: %v", byBoard)
	}
	byColumn, err := taskRepo.ListTasksForColumn(testCtx, colB.ID)
	must(t, err)
	if len(byColumn) != 1 {
		t.Fatalf("unexpected column tasks: %v", byColumn)
	}
	byWorkspace, err := taskRepo.ListTasksForWorkspace(testCtx, ws.ID)
	must(t, err)
	if len(byWorkspace) != 2 {
		t.Fatalf("expected 2 workspace tasks, got %d", len(byWorkspace))
	}

	byIDs, err := taskRepo.FindTasksByIDs(testCtx, []string{byBoard[0].ID, newID()})
	must(t, err)
	if len(byIDs) != 1 {
		t.Fatalf("expected 1 task by ids, got %d", len(byIDs))
	}
	empty, err := taskRepo.FindTasksByIDs(testCtx, nil)
	must(t, err)
	if len(empty) != 0 {
		t.Fatalf("expected empty map for nil ids")
	}

	child := &models.Task{ID: newID(), ColumnID: colA.ID, ParentID: byBoard[0].ID, Title: "Child"}
	must(t, taskRepo.CreateTask(testCtx, child))
	children, err := taskRepo.ListChildTasks(testCtx, byBoard[0].ID)
	must(t, err)
	if len(children) != 1 || children[0].ID != child.ID {
		t.Fatalf("unexpected children: %v", children)
	}
}

func TestTaskRepoDeleteCascades(t *testing.T) {
	db := newTestDB(t)
	taskRepo := NewTaskRepo(db)
	columnRepo := NewColumnRepo(db)
	boardRepo := NewBoardRepo(db)
	attachmentRepo := NewAttachmentRepo(db)
	userRepo := NewUserRepo(db)

	ws := newWorkspace(newID(), "Team")
	must(t, NewWorkspaceRepo(db).CreateWorkspace(testCtx, ws))
	board := newBoard(ws.ID, "Board", 0, true)
	must(t, boardRepo.CreateBoard(testCtx, board))
	column := &models.Column{ID: newID(), BoardID: board.ID, Name: "Backlog"}
	must(t, columnRepo.CreateColumn(testCtx, column))
	u := newUser(newID(), "author")
	must(t, userRepo.Create(testCtx, u))

	parent := &models.Task{ID: newID(), ColumnID: column.ID, Title: "Parent"}
	must(t, taskRepo.CreateTask(testCtx, parent))
	child := &models.Task{ID: newID(), ColumnID: column.ID, ParentID: parent.ID, Title: "Child"}
	must(t, taskRepo.CreateTask(testCtx, child))
	must(t, attachmentRepo.CreateAttachment(testCtx, &models.TaskAttachment{ID: newID(), TaskID: parent.ID, Filename: "f", ObjectKey: "attachments/p", Size: 1, UploadedBy: u.ID}))
	must(t, attachmentRepo.CreateAttachment(testCtx, &models.TaskAttachment{ID: newID(), TaskID: child.ID, Filename: "c", ObjectKey: "attachments/c", Size: 1, UploadedBy: u.ID}))

	must(t, taskRepo.DeleteTask(testCtx, parent.ID))

	mustNotFound(t, errOf(taskRepo.FindTaskByID(testCtx, parent.ID)))
	mustNotFound(t, errOf(taskRepo.FindTaskByID(testCtx, child.ID)))
	keys, err := taskRepo.CollectTaskKeys(testCtx, parent.ID)
	must(t, err)
	if len(keys) != 0 {
		t.Fatalf("task attachments not cleaned: %v", keys)
	}

	mustNotFound(t, taskRepo.DeleteTask(testCtx, newID()))
}

func TestTaskRepoCollectKeys(t *testing.T) {
	db := newTestDB(t)
	taskRepo := NewTaskRepo(db)
	columnRepo := NewColumnRepo(db)
	boardRepo := NewBoardRepo(db)
	attachmentRepo := NewAttachmentRepo(db)
	userRepo := NewUserRepo(db)

	ws := newWorkspace(newID(), "Team")
	must(t, NewWorkspaceRepo(db).CreateWorkspace(testCtx, ws))
	board := newBoard(ws.ID, "Board", 0, true)
	must(t, boardRepo.CreateBoard(testCtx, board))
	column := &models.Column{ID: newID(), BoardID: board.ID, Name: "Backlog"}
	must(t, columnRepo.CreateColumn(testCtx, column))
	u := newUser(newID(), "author")
	must(t, userRepo.Create(testCtx, u))

	parent := &models.Task{ID: newID(), ColumnID: column.ID, Title: "Parent", ImageKey: "tasks/p.png"}
	must(t, taskRepo.CreateTask(testCtx, parent))
	child := &models.Task{ID: newID(), ColumnID: column.ID, ParentID: parent.ID, Title: "Child", ImageKey: "tasks/c.png"}
	must(t, taskRepo.CreateTask(testCtx, child))
	sibling := &models.Task{ID: newID(), ColumnID: column.ID, Title: "Sibling", ImageKey: "tasks/s.png"}
	must(t, taskRepo.CreateTask(testCtx, sibling))
	must(t, attachmentRepo.CreateAttachment(testCtx, &models.TaskAttachment{ID: newID(), TaskID: parent.ID, Filename: "p", ObjectKey: "attachments/p", Size: 1, UploadedBy: u.ID}))
	must(t, attachmentRepo.CreateAttachment(testCtx, &models.TaskAttachment{ID: newID(), TaskID: child.ID, Filename: "c", ObjectKey: "attachments/c", Size: 1, UploadedBy: u.ID}))
	must(t, attachmentRepo.CreateAttachment(testCtx, &models.TaskAttachment{ID: newID(), TaskID: sibling.ID, Filename: "s", ObjectKey: "attachments/s", Size: 1, UploadedBy: u.ID}))

	taskKeys, err := taskRepo.CollectTaskKeys(testCtx, parent.ID)
	must(t, err)
	if len(taskKeys) != 4 {
		t.Fatalf("expected 4 keys for task scope, got %v", taskKeys)
	}
	boardKeys, err := taskRepo.CollectBoardKeys(testCtx, board.ID)
	must(t, err)
	if len(boardKeys) != 6 {
		t.Fatalf("expected 6 keys for board scope, got %v", boardKeys)
	}
	workspaceKeys, err := taskRepo.CollectWorkspaceKeys(testCtx, ws.ID)
	must(t, err)
	if len(workspaceKeys) != 6 {
		t.Fatalf("expected 6 keys for workspace scope, got %v", workspaceKeys)
	}

	must(t, taskRepo.DeleteTask(testCtx, parent.ID))
	remaining, err := taskRepo.CollectBoardKeys(testCtx, board.ID)
	must(t, err)
	if len(remaining) != 2 {
		t.Fatalf("expected 2 keys after delete, got %v", remaining)
	}
}
