package repository

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/repository/entity"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const testTruncate = "TRUNCATE users, workspaces, workspace_members, boards, board_columns, tasks, task_attachments, favorites CASCADE"

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping repository integration tests")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	if err := db.AutoMigrate(
		&entity.User{}, &entity.Workspace{}, &entity.WorkspaceMember{},
		&entity.Board{}, &entity.Column{}, &entity.Task{},
		&entity.TaskAttachment{}, &entity.Favorite{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Exec(testTruncate).Error; err != nil {
			t.Logf("cleanup truncate: %v", err)
		}
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	if err := db.Exec(testTruncate).Error; err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return db
}

func newID() string {
	return uuid.NewString()
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func mustNotFound(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func mustConflict(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, pkgerrors.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func errOf(_ interface{}, err error) error {
	return err
}

var testCtx = context.Background()
