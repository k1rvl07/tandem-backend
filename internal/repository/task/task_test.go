package task

import (
	"fmt"
	"testing"
	"time"

	mattachment "github.com/tandem/tandem/internal/domain/models/attachment"
	mboard "github.com/tandem/tandem/internal/domain/models/board"
	mcolumn "github.com/tandem/tandem/internal/domain/models/column"
	mtask "github.com/tandem/tandem/internal/domain/models/task"
	"github.com/tandem/tandem/internal/repository/board"
	"github.com/tandem/tandem/internal/repository/column"
	"github.com/tandem/tandem/internal/repository/taskext"
	"github.com/tandem/tandem/internal/repository/testutil"
	"github.com/tandem/tandem/internal/repository/user"
	"github.com/tandem/tandem/internal/repository/workspace"
)

type taskFixture struct {
	taskRepo       *TaskRepo
	columnRepo     *column.ColumnRepo
	boardRepo      *board.BoardRepo
	attachmentRepo *taskext.AttachmentRepo
	userRepo       *user.UserRepo
	wsID           string
	board          *mboard.Board
	column         *mcolumn.Column
	boards         []*mboard.Board
	columns        []*mcolumn.Column
	task           *mtask.Task
	tasks          []*mtask.Task
}

func newTaskFixture(t *testing.T, boardCount int) *taskFixture {
	t.Helper()
	db := testutil.NewTestDB(t)
	fx := &taskFixture{
		taskRepo:       NewTaskRepo(db),
		columnRepo:     column.NewColumnRepo(db),
		boardRepo:      board.NewBoardRepo(db),
		attachmentRepo: taskext.NewAttachmentRepo(db),
		userRepo:       user.NewUserRepo(db),
	}
	ws := testutil.NewWorkspace(testutil.NewID(), "Team")
	testutil.Must(t, workspace.NewWorkspaceRepo(db).CreateWorkspace(testutil.TestCtx, ws))
	fx.wsID = ws.ID
	for i := 0; i < boardCount; i++ {
		bd := testutil.NewBoard(ws.ID, fmt.Sprintf("Board %d", i+1), i, i == 0)
		testutil.Must(t, fx.boardRepo.CreateBoard(testutil.TestCtx, bd))
		col := &mcolumn.Column{ID: testutil.NewID(), BoardID: bd.ID, Name: "Backlog"}
		testutil.Must(t, fx.columnRepo.CreateColumn(testutil.TestCtx, col))
		fx.boards = append(fx.boards, bd)
		fx.columns = append(fx.columns, col)
		if i == 0 {
			fx.board = bd
			fx.column = col
		}
	}
	return fx
}

func TestTaskRepoCRUD(t *testing.T) {
	fx := newTaskFixture(t, 1)

	t.Run("creates and reads a task", func(t *testing.T) {
		due := time.Now().Add(24 * time.Hour).Truncate(time.Second)
		fx.task = &mtask.Task{ID: testutil.NewID(), ColumnID: fx.column.ID, Title: "Task", Description: "desc", Position: 0, IsUrgent: true, DueDate: &due}
		testutil.Must(t, fx.taskRepo.CreateTask(testutil.TestCtx, fx.task))
		if fx.task.CreatedAt.IsZero() {
			t.Fatalf("created_at not set")
		}
		got, err := fx.taskRepo.FindTaskByID(testutil.TestCtx, fx.task.ID)
		testutil.Must(t, err)
		if got.Title != "Task" || !got.IsUrgent || got.DueDate == nil || got.DueDate.Sub(due) > time.Second {
			t.Fatalf("unexpected task: %+v", got)
		}
		testutil.MustNotFound(t, testutil.ErrOf(fx.taskRepo.FindTaskByID(testutil.TestCtx, testutil.NewID())))
	})

	t.Run("updates a task", func(t *testing.T) {
		tk := fx.task
		tk.Title = "Updated"
		tk.Position = 2
		tk.DueDate = nil
		tk.IsUrgent = false
		testutil.Must(t, fx.taskRepo.UpdateTask(testutil.TestCtx, tk))
		got, err := fx.taskRepo.FindTaskByID(testutil.TestCtx, tk.ID)
		testutil.Must(t, err)
		if got.Title != "Updated" || got.DueDate != nil || got.IsUrgent {
			t.Fatalf("update not applied: %+v", got)
		}
	})
}

func TestTaskRepoLists(t *testing.T) {
	fx := newTaskFixture(t, 2)

	t.Run("creates one task per column", func(t *testing.T) {
		for i, col := range fx.columns {
			task := &mtask.Task{ID: testutil.NewID(), ColumnID: col.ID, Title: "T", Position: i}
			testutil.Must(t, fx.taskRepo.CreateTask(testutil.TestCtx, task))
			fx.tasks = append(fx.tasks, task)
		}
	})

	t.Run("lists tasks for board, column, and workspace", func(t *testing.T) {
		byBoard, err := fx.taskRepo.ListTasksForBoard(testutil.TestCtx, fx.boards[0].ID)
		testutil.Must(t, err)
		if len(byBoard) != 1 || byBoard[0].ColumnID != fx.columns[0].ID {
			t.Fatalf("unexpected board tasks: %v", byBoard)
		}
		byColumn, err := fx.taskRepo.ListTasksForColumn(testutil.TestCtx, fx.columns[1].ID)
		testutil.Must(t, err)
		if len(byColumn) != 1 {
			t.Fatalf("unexpected column tasks: %v", byColumn)
		}
		byWorkspace, err := fx.taskRepo.ListTasksForWorkspace(testutil.TestCtx, fx.wsID)
		testutil.Must(t, err)
		if len(byWorkspace) != 2 {
			t.Fatalf("expected 2 workspace tasks, got %d", len(byWorkspace))
		}
	})

	t.Run("finds tasks by ids", func(t *testing.T) {
		byIDs, err := fx.taskRepo.FindTasksByIDs(testutil.TestCtx, []string{fx.tasks[0].ID, testutil.NewID()})
		testutil.Must(t, err)
		if len(byIDs) != 1 {
			t.Fatalf("expected 1 task by ids, got %d", len(byIDs))
		}
		empty, err := fx.taskRepo.FindTasksByIDs(testutil.TestCtx, nil)
		testutil.Must(t, err)
		if len(empty) != 0 {
			t.Fatalf("expected empty map for nil ids")
		}
	})

	t.Run("lists child tasks", func(t *testing.T) {
		child := &mtask.Task{ID: testutil.NewID(), ColumnID: fx.columns[0].ID, ParentID: fx.tasks[0].ID, Title: "Child"}
		testutil.Must(t, fx.taskRepo.CreateTask(testutil.TestCtx, child))
		children, err := fx.taskRepo.ListChildTasks(testutil.TestCtx, fx.tasks[0].ID)
		testutil.Must(t, err)
		if len(children) != 1 || children[0].ID != child.ID {
			t.Fatalf("unexpected children: %v", children)
		}
	})
}

func TestTaskRepoDeleteCascades(t *testing.T) {
	fx := newTaskFixture(t, 1)
	u := testutil.NewUser(testutil.NewID(), "author")
	testutil.Must(t, fx.userRepo.Create(testutil.TestCtx, u))

	t.Run("cascades delete to children and attachments", func(t *testing.T) {
		parent := &mtask.Task{ID: testutil.NewID(), ColumnID: fx.column.ID, Title: "Parent"}
		testutil.Must(t, fx.taskRepo.CreateTask(testutil.TestCtx, parent))
		child := &mtask.Task{ID: testutil.NewID(), ColumnID: fx.column.ID, ParentID: parent.ID, Title: "Child"}
		testutil.Must(t, fx.taskRepo.CreateTask(testutil.TestCtx, child))
		testutil.Must(t, fx.attachmentRepo.CreateAttachment(testutil.TestCtx, &mattachment.TaskAttachment{ID: testutil.NewID(), TaskID: parent.ID, Filename: "f", ObjectKey: "attachments/p", Size: 1, UploadedBy: u.ID}))
		testutil.Must(t, fx.attachmentRepo.CreateAttachment(testutil.TestCtx, &mattachment.TaskAttachment{ID: testutil.NewID(), TaskID: child.ID, Filename: "c", ObjectKey: "attachments/c", Size: 1, UploadedBy: u.ID}))

		testutil.Must(t, fx.taskRepo.DeleteTask(testutil.TestCtx, parent.ID))

		testutil.MustNotFound(t, testutil.ErrOf(fx.taskRepo.FindTaskByID(testutil.TestCtx, parent.ID)))
		testutil.MustNotFound(t, testutil.ErrOf(fx.taskRepo.FindTaskByID(testutil.TestCtx, child.ID)))
		keys, err := fx.taskRepo.CollectTaskKeys(testutil.TestCtx, parent.ID)
		testutil.Must(t, err)
		if len(keys) != 0 {
			t.Fatalf("task attachments not cleaned: %v", keys)
		}
	})

	t.Run("delete missing task is not found", func(t *testing.T) {
		testutil.MustNotFound(t, fx.taskRepo.DeleteTask(testutil.TestCtx, testutil.NewID()))
	})
}

func TestTaskRepoMove(t *testing.T) {
	fx := newTaskFixture(t, 2)
	tasks := make([]*mtask.Task, 3)
	for i := range tasks {
		tasks[i] = &mtask.Task{ID: testutil.NewID(), ColumnID: fx.columns[0].ID, Title: fmt.Sprintf("T%d", i), Position: i}
		testutil.Must(t, fx.taskRepo.CreateTask(testutil.TestCtx, tasks[i]))
	}

	t.Run("reorders within the same column", func(t *testing.T) {
		moved := &mtask.Task{ID: tasks[2].ID, ColumnID: fx.columns[0].ID}
		testutil.Must(t, fx.taskRepo.MoveTask(testutil.TestCtx, moved, fx.columns[0].ID, 0))
		order, err := fx.taskRepo.ListTasksForColumn(testutil.TestCtx, fx.columns[0].ID)
		testutil.Must(t, err)
		if len(order) != 3 || order[0].ID != tasks[2].ID || order[1].ID != tasks[0].ID || order[2].ID != tasks[1].ID {
			t.Fatalf("unexpected order after same-column move: %v", idsOf(order))
		}
		if moved.Position != 0 {
			t.Fatalf("expected moved task position 0, got %d", moved.Position)
		}
	})

	t.Run("moves across columns and compacts source order", func(t *testing.T) {
		moved := &mtask.Task{ID: tasks[0].ID, ColumnID: fx.columns[0].ID}
		testutil.Must(t, fx.taskRepo.MoveTask(testutil.TestCtx, moved, fx.columns[1].ID, 0))
		if moved.ColumnID != fx.columns[1].ID {
			t.Fatalf("expected task column %s, got %s", fx.columns[1].ID, moved.ColumnID)
		}
		src, err := fx.taskRepo.ListTasksForColumn(testutil.TestCtx, fx.columns[0].ID)
		testutil.Must(t, err)
		if len(src) != 2 || src[0].ID != tasks[2].ID || src[1].ID != tasks[1].ID {
			t.Fatalf("unexpected source column: %v", idsOf(src))
		}
		dst, err := fx.taskRepo.ListTasksForColumn(testutil.TestCtx, fx.columns[1].ID)
		testutil.Must(t, err)
		if len(dst) != 1 || dst[0].ID != tasks[0].ID {
			t.Fatalf("unexpected target column: %v", idsOf(dst))
		}
	})

	t.Run("moving a missing task is not found", func(t *testing.T) {
		ghost := &mtask.Task{ID: testutil.NewID(), ColumnID: fx.columns[0].ID}
		testutil.MustNotFound(t, fx.taskRepo.MoveTask(testutil.TestCtx, ghost, fx.columns[1].ID, 0))
	})
}

func idsOf(tasks []*mtask.Task) []string {
	ids := make([]string, 0, len(tasks))
	for _, t := range tasks {
		ids = append(ids, t.ID)
	}
	return ids
}

func TestTaskRepoCollectKeys(t *testing.T) {
	fx := newTaskFixture(t, 1)
	u := testutil.NewUser(testutil.NewID(), "author")
	testutil.Must(t, fx.userRepo.Create(testutil.TestCtx, u))

	t.Run("collects and prunes object keys", func(t *testing.T) {
		parent := &mtask.Task{ID: testutil.NewID(), ColumnID: fx.column.ID, Title: "Parent", ImageKey: "tasks/p.png"}
		testutil.Must(t, fx.taskRepo.CreateTask(testutil.TestCtx, parent))
		child := &mtask.Task{ID: testutil.NewID(), ColumnID: fx.column.ID, ParentID: parent.ID, Title: "Child", ImageKey: "tasks/c.png"}
		testutil.Must(t, fx.taskRepo.CreateTask(testutil.TestCtx, child))
		sibling := &mtask.Task{ID: testutil.NewID(), ColumnID: fx.column.ID, Title: "Sibling", ImageKey: "tasks/s.png"}
		testutil.Must(t, fx.taskRepo.CreateTask(testutil.TestCtx, sibling))
		testutil.Must(t, fx.attachmentRepo.CreateAttachment(testutil.TestCtx, &mattachment.TaskAttachment{ID: testutil.NewID(), TaskID: parent.ID, Filename: "p", ObjectKey: "attachments/p", Size: 1, UploadedBy: u.ID}))
		testutil.Must(t, fx.attachmentRepo.CreateAttachment(testutil.TestCtx, &mattachment.TaskAttachment{ID: testutil.NewID(), TaskID: child.ID, Filename: "c", ObjectKey: "attachments/c", Size: 1, UploadedBy: u.ID}))
		testutil.Must(t, fx.attachmentRepo.CreateAttachment(testutil.TestCtx, &mattachment.TaskAttachment{ID: testutil.NewID(), TaskID: sibling.ID, Filename: "s", ObjectKey: "attachments/s", Size: 1, UploadedBy: u.ID}))

		taskKeys, err := fx.taskRepo.CollectTaskKeys(testutil.TestCtx, parent.ID)
		testutil.Must(t, err)
		if len(taskKeys) != 4 {
			t.Fatalf("expected 4 keys for task scope, got %v", taskKeys)
		}
		boardKeys, err := fx.taskRepo.CollectBoardKeys(testutil.TestCtx, fx.board.ID)
		testutil.Must(t, err)
		if len(boardKeys) != 6 {
			t.Fatalf("expected 6 keys for board scope, got %v", boardKeys)
		}
		workspaceKeys, err := fx.taskRepo.CollectWorkspaceKeys(testutil.TestCtx, fx.wsID)
		testutil.Must(t, err)
		if len(workspaceKeys) != 6 {
			t.Fatalf("expected 6 keys for workspace scope, got %v", workspaceKeys)
		}

		testutil.Must(t, fx.taskRepo.DeleteTask(testutil.TestCtx, parent.ID))
		remaining, err := fx.taskRepo.CollectBoardKeys(testutil.TestCtx, fx.board.ID)
		testutil.Must(t, err)
		if len(remaining) != 2 {
			t.Fatalf("expected 2 keys after delete, got %v", remaining)
		}
	})
}
