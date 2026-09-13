package testutil_test

import (
	"testing"

	"gorm.io/gorm"

	etask "github.com/tandem/tandem/internal/repository/entity/task"
	"github.com/tandem/tandem/internal/repository/testutil"
)

func insertRow(t *testing.T, db *gorm.DB, query string, args ...any) {
	t.Helper()
	if err := db.Exec(query, args...).Error; err != nil {
		t.Fatalf("insert: %v", err)
	}
}

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

	inserts := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO users (id, login, password_hash, role, display_name) VALUES (?, ?, ?, ?, ?)`, []any{userID, "owner", "hash", "user", "Owner"}},
		{`INSERT INTO workspaces (id, name, description, prefix, theme, created_at, updated_at) VALUES (?, ?, ?, ?, ?, now(), now())`, []any{wsID, "WS", "d", "TA", "blue"}},
		{`INSERT INTO workspace_members (workspace_id, user_id, role, created_at) VALUES (?, ?, ?, now())`, []any{wsID, userID, "owner"}},
		{`INSERT INTO boards (id, workspace_id, name, position, is_main, created_at, updated_at) VALUES (?, ?, ?, ?, ?, now(), now())`, []any{boardID, wsID, "Board", 0, true}},
		{`INSERT INTO board_columns (id, board_id, name, position, created_at, updated_at) VALUES (?, ?, ?, ?, now(), now())`, []any{columnID, boardID, "Col", 0}},
		{`INSERT INTO tasks (id, column_id, title, author_id, position, created_at, updated_at) VALUES (?, ?, ?, ?, 0, now(), now())`, []any{parentTaskID, columnID, "Parent", userID}},
		{`INSERT INTO tasks (id, column_id, title, parent_id, author_id, position, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 0, now(), now())`, []any{taskID, columnID, "Child", parentTaskID, userID}},
		{`INSERT INTO task_attachments (id, task_id, filename, object_key, size, content_type, uploaded_by, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, now())`, []any{attachmentID, taskID, "f.txt", "k", 10, "text/plain", userID}},
	}
	for _, in := range inserts {
		insertRow(t, db, in.query, in.args...)
	}

	insertRow(t, db, `DELETE FROM workspaces WHERE id = ?`, wsID)

	for _, table := range []string{"tasks", "task_attachments", "board_columns", "boards", "workspace_members"} {
		var remaining int64
		db.Table(table).Count(&remaining)
		if remaining != 0 {
			t.Fatalf("expected %s cascade deleted, %d left", table, remaining)
		}
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

	for _, u := range []struct{ id, login string }{
		{authorID, "author"},
		{assigneeID, "assignee"},
		{curatorID, "curator"},
	} {
		insertRow(t, db, `INSERT INTO users (id, login, password_hash, role, display_name) VALUES (?, ?, ?, ?, ?)`, u.id, u.login, "hash", "user", "U"+u.login)
	}

	insertRow(t, db, `INSERT INTO workspaces (id, name, description, prefix, theme, created_at, updated_at) VALUES (?, ?, ?, ?, ?, now(), now())`, wsID, "WS", "d", "TA", "blue")
	insertRow(t, db, `INSERT INTO boards (id, workspace_id, name, position, is_main, created_at, updated_at) VALUES (?, ?, ?, ?, ?, now(), now())`, boardID, wsID, "Board", 0, true)
	insertRow(t, db, `INSERT INTO board_columns (id, board_id, name, position, created_at, updated_at) VALUES (?, ?, ?, ?, now(), now())`, columnID, boardID, "Col", 0)
	insertRow(t, db, `INSERT INTO tasks (id, column_id, title, author_id, assignee_id, curator_id, position, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, 0, now(), now())`, taskID, columnID, "Task", authorID, assigneeID, curatorID)
	insertRow(t, db, `INSERT INTO task_attachments (id, task_id, filename, object_key, size, content_type, uploaded_by, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, now())`, testutil.NewID(), taskID, "f.txt", "k", 10, "text/plain", authorID)

	insertRow(t, db, `DELETE FROM users WHERE id = ?`, authorID)

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
