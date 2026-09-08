package task

import (
	"testing"
	"time"

	mattachment "github.com/tandem/tandem/internal/domain/models/attachment"
	mcolumn "github.com/tandem/tandem/internal/domain/models/column"
	mtask "github.com/tandem/tandem/internal/domain/models/task"
	"github.com/tandem/tandem/internal/repository/board"
	"github.com/tandem/tandem/internal/repository/column"
	"github.com/tandem/tandem/internal/repository/taskext"
	"github.com/tandem/tandem/internal/repository/testutil"
	"github.com/tandem/tandem/internal/repository/user"
	"github.com/tandem/tandem/internal/repository/workspace"
)

func TestTaskRepoCRUD(t *testing.T) {
	db := testutil.NewTestDB(t)
	taskRepo := NewTaskRepo(db)
	columnRepo := column.NewColumnRepo(db)
	boardRepo := board.NewBoardRepo(db)

	ws := testutil.NewWorkspace(testutil.NewID(), "Team")
	testutil.Must(t, workspace.NewWorkspaceRepo(db).CreateWorkspace(testutil.TestCtx, ws))
	board := testutil.NewBoard(ws.ID, "Board", 0, true)
	testutil.Must(t, boardRepo.CreateBoard(testutil.TestCtx, board))
	column := &mcolumn.Column{ID: testutil.NewID(), BoardID: board.ID, Name: "Backlog", Position: 0}
	testutil.Must(t, columnRepo.CreateColumn(testutil.TestCtx, column))

	due := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	tk := &mtask.Task{ID: testutil.NewID(), ColumnID: column.ID, Title: "Task", Description: "desc", Position: 0, IsUrgent: true, DueDate: &due}
	testutil.Must(t, taskRepo.CreateTask(testutil.TestCtx, tk))
	if tk.CreatedAt.IsZero() {
		t.Fatalf("created_at not set")
	}

	got, err := taskRepo.FindTaskByID(testutil.TestCtx, tk.ID)
	testutil.Must(t, err)
	if got.Title != "Task" || !got.IsUrgent || got.DueDate == nil || got.DueDate.Sub(due) > time.Second {
		t.Fatalf("unexpected task: %+v", got)
	}
	testutil.MustNotFound(t, testutil.ErrOf(taskRepo.FindTaskByID(testutil.TestCtx, testutil.NewID())))

	tk.Title = "Updated"
	tk.Position = 2
	tk.DueDate = nil
	tk.IsUrgent = false
	testutil.Must(t, taskRepo.UpdateTask(testutil.TestCtx, tk))
	got, err = taskRepo.FindTaskByID(testutil.TestCtx, tk.ID)
	testutil.Must(t, err)
	if got.Title != "Updated" || got.DueDate != nil || got.IsUrgent {
		t.Fatalf("update not applied: %+v", got)
	}
}

func TestTaskRepoLists(t *testing.T) {
	db := testutil.NewTestDB(t)
	taskRepo := NewTaskRepo(db)
	columnRepo := column.NewColumnRepo(db)
	boardRepo := board.NewBoardRepo(db)

	ws := testutil.NewWorkspace(testutil.NewID(), "Team")
	testutil.Must(t, workspace.NewWorkspaceRepo(db).CreateWorkspace(testutil.TestCtx, ws))
	boardA := testutil.NewBoard(ws.ID, "A", 0, true)
	boardB := testutil.NewBoard(ws.ID, "B", 1, false)
	testutil.Must(t, boardRepo.CreateBoard(testutil.TestCtx, boardA))
	testutil.Must(t, boardRepo.CreateBoard(testutil.TestCtx, boardB))
	colA := &mcolumn.Column{ID: testutil.NewID(), BoardID: boardA.ID, Name: "Backlog"}
	colB := &mcolumn.Column{ID: testutil.NewID(), BoardID: boardB.ID, Name: "Done"}
	testutil.Must(t, columnRepo.CreateColumn(testutil.TestCtx, colA))
	testutil.Must(t, columnRepo.CreateColumn(testutil.TestCtx, colB))

	for i, col := range []*mcolumn.Column{colA, colB} {
		task := &mtask.Task{ID: testutil.NewID(), ColumnID: col.ID, Title: "T", Position: i}
		testutil.Must(t, taskRepo.CreateTask(testutil.TestCtx, task))
	}

	byBoard, err := taskRepo.ListTasksForBoard(testutil.TestCtx, boardA.ID)
	testutil.Must(t, err)
	if len(byBoard) != 1 || byBoard[0].ColumnID != colA.ID {
		t.Fatalf("unexpected board tasks: %v", byBoard)
	}
	byColumn, err := taskRepo.ListTasksForColumn(testutil.TestCtx, colB.ID)
	testutil.Must(t, err)
	if len(byColumn) != 1 {
		t.Fatalf("unexpected column tasks: %v", byColumn)
	}
	byWorkspace, err := taskRepo.ListTasksForWorkspace(testutil.TestCtx, ws.ID)
	testutil.Must(t, err)
	if len(byWorkspace) != 2 {
		t.Fatalf("expected 2 workspace tasks, got %d", len(byWorkspace))
	}

	byIDs, err := taskRepo.FindTasksByIDs(testutil.TestCtx, []string{byBoard[0].ID, testutil.NewID()})
	testutil.Must(t, err)
	if len(byIDs) != 1 {
		t.Fatalf("expected 1 task by ids, got %d", len(byIDs))
	}
	empty, err := taskRepo.FindTasksByIDs(testutil.TestCtx, nil)
	testutil.Must(t, err)
	if len(empty) != 0 {
		t.Fatalf("expected empty map for nil ids")
	}

	child := &mtask.Task{ID: testutil.NewID(), ColumnID: colA.ID, ParentID: byBoard[0].ID, Title: "Child"}
	testutil.Must(t, taskRepo.CreateTask(testutil.TestCtx, child))
	children, err := taskRepo.ListChildTasks(testutil.TestCtx, byBoard[0].ID)
	testutil.Must(t, err)
	if len(children) != 1 || children[0].ID != child.ID {
		t.Fatalf("unexpected children: %v", children)
	}
}

func TestTaskRepoDeleteCascades(t *testing.T) {
	db := testutil.NewTestDB(t)
	taskRepo := NewTaskRepo(db)
	columnRepo := column.NewColumnRepo(db)
	boardRepo := board.NewBoardRepo(db)
	attachmentRepo := taskext.NewAttachmentRepo(db)
	userRepo := user.NewUserRepo(db)

	ws := testutil.NewWorkspace(testutil.NewID(), "Team")
	testutil.Must(t, workspace.NewWorkspaceRepo(db).CreateWorkspace(testutil.TestCtx, ws))
	board := testutil.NewBoard(ws.ID, "Board", 0, true)
	testutil.Must(t, boardRepo.CreateBoard(testutil.TestCtx, board))
	column := &mcolumn.Column{ID: testutil.NewID(), BoardID: board.ID, Name: "Backlog"}
	testutil.Must(t, columnRepo.CreateColumn(testutil.TestCtx, column))
	u := testutil.NewUser(testutil.NewID(), "author")
	testutil.Must(t, userRepo.Create(testutil.TestCtx, u))

	parent := &mtask.Task{ID: testutil.NewID(), ColumnID: column.ID, Title: "Parent"}
	testutil.Must(t, taskRepo.CreateTask(testutil.TestCtx, parent))
	child := &mtask.Task{ID: testutil.NewID(), ColumnID: column.ID, ParentID: parent.ID, Title: "Child"}
	testutil.Must(t, taskRepo.CreateTask(testutil.TestCtx, child))
	testutil.Must(t, attachmentRepo.CreateAttachment(testutil.TestCtx, &mattachment.TaskAttachment{ID: testutil.NewID(), TaskID: parent.ID, Filename: "f", ObjectKey: "attachments/p", Size: 1, UploadedBy: u.ID}))
	testutil.Must(t, attachmentRepo.CreateAttachment(testutil.TestCtx, &mattachment.TaskAttachment{ID: testutil.NewID(), TaskID: child.ID, Filename: "c", ObjectKey: "attachments/c", Size: 1, UploadedBy: u.ID}))

	testutil.Must(t, taskRepo.DeleteTask(testutil.TestCtx, parent.ID))

	testutil.MustNotFound(t, testutil.ErrOf(taskRepo.FindTaskByID(testutil.TestCtx, parent.ID)))
	testutil.MustNotFound(t, testutil.ErrOf(taskRepo.FindTaskByID(testutil.TestCtx, child.ID)))
	keys, err := taskRepo.CollectTaskKeys(testutil.TestCtx, parent.ID)
	testutil.Must(t, err)
	if len(keys) != 0 {
		t.Fatalf("task attachments not cleaned: %v", keys)
	}

	testutil.MustNotFound(t, taskRepo.DeleteTask(testutil.TestCtx, testutil.NewID()))
}

func TestTaskRepoCollectKeys(t *testing.T) {
	db := testutil.NewTestDB(t)
	taskRepo := NewTaskRepo(db)
	columnRepo := column.NewColumnRepo(db)
	boardRepo := board.NewBoardRepo(db)
	attachmentRepo := taskext.NewAttachmentRepo(db)
	userRepo := user.NewUserRepo(db)

	ws := testutil.NewWorkspace(testutil.NewID(), "Team")
	testutil.Must(t, workspace.NewWorkspaceRepo(db).CreateWorkspace(testutil.TestCtx, ws))
	board := testutil.NewBoard(ws.ID, "Board", 0, true)
	testutil.Must(t, boardRepo.CreateBoard(testutil.TestCtx, board))
	column := &mcolumn.Column{ID: testutil.NewID(), BoardID: board.ID, Name: "Backlog"}
	testutil.Must(t, columnRepo.CreateColumn(testutil.TestCtx, column))
	u := testutil.NewUser(testutil.NewID(), "author")
	testutil.Must(t, userRepo.Create(testutil.TestCtx, u))

	parent := &mtask.Task{ID: testutil.NewID(), ColumnID: column.ID, Title: "Parent", ImageKey: "tasks/p.png"}
	testutil.Must(t, taskRepo.CreateTask(testutil.TestCtx, parent))
	child := &mtask.Task{ID: testutil.NewID(), ColumnID: column.ID, ParentID: parent.ID, Title: "Child", ImageKey: "tasks/c.png"}
	testutil.Must(t, taskRepo.CreateTask(testutil.TestCtx, child))
	sibling := &mtask.Task{ID: testutil.NewID(), ColumnID: column.ID, Title: "Sibling", ImageKey: "tasks/s.png"}
	testutil.Must(t, taskRepo.CreateTask(testutil.TestCtx, sibling))
	testutil.Must(t, attachmentRepo.CreateAttachment(testutil.TestCtx, &mattachment.TaskAttachment{ID: testutil.NewID(), TaskID: parent.ID, Filename: "p", ObjectKey: "attachments/p", Size: 1, UploadedBy: u.ID}))
	testutil.Must(t, attachmentRepo.CreateAttachment(testutil.TestCtx, &mattachment.TaskAttachment{ID: testutil.NewID(), TaskID: child.ID, Filename: "c", ObjectKey: "attachments/c", Size: 1, UploadedBy: u.ID}))
	testutil.Must(t, attachmentRepo.CreateAttachment(testutil.TestCtx, &mattachment.TaskAttachment{ID: testutil.NewID(), TaskID: sibling.ID, Filename: "s", ObjectKey: "attachments/s", Size: 1, UploadedBy: u.ID}))

	taskKeys, err := taskRepo.CollectTaskKeys(testutil.TestCtx, parent.ID)
	testutil.Must(t, err)
	if len(taskKeys) != 4 {
		t.Fatalf("expected 4 keys for task scope, got %v", taskKeys)
	}
	boardKeys, err := taskRepo.CollectBoardKeys(testutil.TestCtx, board.ID)
	testutil.Must(t, err)
	if len(boardKeys) != 6 {
		t.Fatalf("expected 6 keys for board scope, got %v", boardKeys)
	}
	workspaceKeys, err := taskRepo.CollectWorkspaceKeys(testutil.TestCtx, ws.ID)
	testutil.Must(t, err)
	if len(workspaceKeys) != 6 {
		t.Fatalf("expected 6 keys for workspace scope, got %v", workspaceKeys)
	}

	testutil.Must(t, taskRepo.DeleteTask(testutil.TestCtx, parent.ID))
	remaining, err := taskRepo.CollectBoardKeys(testutil.TestCtx, board.ID)
	testutil.Must(t, err)
	if len(remaining) != 2 {
		t.Fatalf("expected 2 keys after delete, got %v", remaining)
	}
}
