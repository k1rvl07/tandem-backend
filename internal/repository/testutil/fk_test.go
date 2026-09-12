package testutil_test

import (
	"testing"

	etask "github.com/tandem/tandem/internal/repository/entity/task"
	"github.com/tandem/tandem/internal/repository/testutil"
)

func TestForeignKeyConstraintsExist(t *testing.T) {
	db := testutil.NewTestDB(t)

	var count int
	err := db.Raw(`
		SELECT count(*) FROM pg_constraint c
		JOIN pg_class ct ON ct.oid = c.conrelid
		WHERE c.contype = 'f' AND contype = 'f' AND ct.relname IN ('boards', 'board_columns', 'tasks', 'task_attachments', 'workspace_members')
	`).Scan(&count).Error
	testutil.Must(t, err)

	if count < 7 {
		t.Fatalf("expected at least 7 foreign keys on child tables, got %d", count)
	}
}

func TestCascadeDeletesFromWorkspace(t *testing.T) {
	db := testutil.NewTestDB(t)

	wsID := testutil.NewID()
	boardID := testutil.NewID()
	columnID := testutil.NewID()
	taskID := testutil.NewID()
	parentTaskID := testutil.NewID()
	userID := testutil.NewID()
	attachmentID := testutil.NewID()

	if err := db.Exec(`INSERT INTO users (id, login, password_hash, role, display_name) VALUES (?, ?, ?, ?, ?)`, userID, "owner", "hash", "user", "Owner").Error; err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if err := db.Exec(`INSERT INTO workspaces (id, name, description, prefix, theme, created_at, updated_at) VALUES (?, ?, ?, ?, ?, now(), now())`, wsID, "WS", "d", "TA", "blue").Error; err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	if err := db.Exec(`INSERT INTO workspace_members (workspace_id, user_id, role, created_at) VALUES (?, ?, ?, now())`, wsID, userID, "owner").Error; err != nil {
		t.Fatalf("insert member: %v", err)
	}
	if err := db.Exec(`INSERT INTO boards (id, workspace_id, name, position, is_main, created_at, updated_at) VALUES (?, ?, ?, ?, ?, now(), now())`, boardID, wsID, "Board", 0, true).Error; err != nil {
		t.Fatalf("insert board: %v", err)
	}
	if err := db.Exec(`INSERT INTO board_columns (id, board_id, name, position, created_at, updated_at) VALUES (?, ?, ?, ?, now(), now())`, columnID, boardID, "Col", 0).Error; err != nil {
		t.Fatalf("insert column: %v", err)
	}
	if err := db.Exec(`INSERT INTO tasks (id, column_id, title, author_id, position, created_at, updated_at) VALUES (?, ?, ?, ?, 0, now(), now())`, parentTaskID, columnID, "Parent", userID).Error; err != nil {
		t.Fatalf("insert parent task: %v", err)
	}
	if err := db.Exec(`INSERT INTO tasks (id, column_id, title, parent_id, author_id, position, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 0, now(), now())`, taskID, columnID, "Child", parentTaskID, userID).Error; err != nil {
		t.Fatalf("insert child task: %v", err)
	}
	if err := db.Exec(`INSERT INTO task_attachments (id, task_id, filename, object_key, size, content_type, uploaded_by, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, now())`, attachmentID, taskID, "f.txt", "k", 10, "text/plain", userID).Error; err != nil {
		t.Fatalf("insert attachment: %v", err)
	}

	if err := db.Exec(`DELETE FROM workspaces WHERE id = ?`, wsID).Error; err != nil {
		t.Fatalf("delete workspace: %v", err)
	}

	var remaining int64
	db.Table("tasks").Count(&remaining)
	if remaining != 0 {
		t.Fatalf("expected tasks cascade deleted, %d left", remaining)
	}
	db.Table("task_attachments").Count(&remaining)
	if remaining != 0 {
		t.Fatalf("expected attachments cascade deleted, %d left", remaining)
	}
	db.Table("board_columns").Count(&remaining)
	if remaining != 0 {
		t.Fatalf("expected columns cascade deleted, %d left", remaining)
	}
	db.Table("boards").Count(&remaining)
	if remaining != 0 {
		t.Fatalf("expected boards cascade deleted, %d left", remaining)
	}
	db.Table("workspace_members").Count(&remaining)
	if remaining != 0 {
		t.Fatalf("expected members cascade deleted, %d left", remaining)
	}
}

func TestDeleteUserSetsNullOnActorColumns(t *testing.T) {
	db := testutil.NewTestDB(t)

	wsID := testutil.NewID()
	boardID := testutil.NewID()
	columnID := testutil.NewID()
	taskID := testutil.NewID()
	authorID := testutil.NewID()
	assigneeID := testutil.NewID()
	curatorID := testutil.NewID()

	insert := func(id, login string, role string) error {
		return db.Exec(`INSERT INTO users (id, login, password_hash, role, display_name) VALUES (?, ?, ?, ?, ?)`, id, login, "hash", role, "U"+login).Error
	}
	for _, u := range []struct{ id, login, role string }{
		{authorID, "author", "user"},
		{assigneeID, "assignee", "user"},
		{curatorID, "curator", "user"},
	} {
		if err := insert(u.id, u.login, u.role); err != nil {
			t.Fatalf("insert user: %v", err)
		}
	}

	if err := db.Exec(`INSERT INTO workspaces (id, name, description, prefix, theme, created_at, updated_at) VALUES (?, ?, ?, ?, ?, now(), now())`, wsID, "WS", "d", "TA", "blue").Error; err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	if err := db.Exec(`INSERT INTO boards (id, workspace_id, name, position, is_main, created_at, updated_at) VALUES (?, ?, ?, ?, ?, now(), now())`, boardID, wsID, "Board", 0, true).Error; err != nil {
		t.Fatalf("insert board: %v", err)
	}
	if err := db.Exec(`INSERT INTO board_columns (id, board_id, name, position, created_at, updated_at) VALUES (?, ?, ?, ?, now(), now())`, columnID, boardID, "Col", 0).Error; err != nil {
		t.Fatalf("insert column: %v", err)
	}
	if err := db.Exec(`INSERT INTO tasks (id, column_id, title, author_id, assignee_id, curator_id, position, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 0, now(), now())`, taskID, columnID, "Task", authorID, assigneeID, curatorID).Error; err != nil {
		t.Fatalf("insert task: %v", err)
	}
	if err := db.Exec(`INSERT INTO task_attachments (id, task_id, filename, object_key, size, content_type, uploaded_by, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, now())`, testutil.NewID(), taskID, "f.txt", "k", 10, "text/plain", authorID).Error; err != nil {
		t.Fatalf("insert attachment: %v", err)
	}

	if err := db.Exec(`DELETE FROM users WHERE id = ?`, authorID).Error; err != nil {
		t.Fatalf("delete author: %v", err)
	}

	var authorIDOut, assigneeIDOut, curatorIDOut *string
	if err := db.Table("tasks").Where("id = ?", taskID).Select("author_id, assignee_id, curator_id").Row().Scan(&authorIDOut, &assigneeIDOut, &curatorIDOut); err != nil {
		t.Fatalf("scan task: %v", err)
	}
	if authorIDOut != nil {
		t.Fatalf("expected author_id NULL after user delete, got %v", *authorIDOut)
	}
	if assigneeIDOut == nil || *assigneeIDOut != assigneeID {
		t.Fatalf("expected assignee untouched, got %v", assigneeIDOut)
	}
	if curatorIDOut == nil || *curatorIDOut != curatorID {
		t.Fatalf("expected curator untouched, got %v", curatorIDOut)
	}

	var uploadedBy *string
	if err := db.Model(&etask.TaskAttachment{}).Where("task_id = ?", taskID).Select("uploaded_by").Row().Scan(&uploadedBy); err != nil {
		t.Fatalf("scan attachment: %v", err)
	}
	if uploadedBy != nil {
		t.Fatalf("expected uploaded_by NULL after user delete, got %v", *uploadedBy)
	}
}
