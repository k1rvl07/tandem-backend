package testutil

import (
	"context"
	"errors"
	"os"
	"sync"
	"syscall"
	"testing"

	"github.com/google/uuid"
	mboard "github.com/tandem/tandem/internal/domain/models/board"
	muser "github.com/tandem/tandem/internal/domain/models/user"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	eboard "github.com/tandem/tandem/internal/repository/entity/board"
	efavorite "github.com/tandem/tandem/internal/repository/entity/favorite"
	etask "github.com/tandem/tandem/internal/repository/entity/task"
	euser "github.com/tandem/tandem/internal/repository/entity/user"
	eworkspace "github.com/tandem/tandem/internal/repository/entity/workspace"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const testTruncate = "TRUNCATE users, workspaces, workspace_members, boards, board_columns, tasks, task_attachments, favorites CASCADE"

func NewTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	serializeTestPackages(t)
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
		&euser.User{}, &eworkspace.Workspace{}, &eworkspace.WorkspaceMember{},
		&eboard.Board{}, &eboard.Column{}, &eboard.Task{},
		&etask.TaskAttachment{}, &efavorite.Favorite{},
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

func NewID() string {
	return uuid.NewString()
}

func NewUser(id, login string) *muser.User {
	return &muser.User{
		ID:           id,
		Login:        login,
		PasswordHash: "hash",
		Role:         muser.RoleUser,
		DisplayName:  "User " + login,
		Bio:          "bio",
		AvatarKey:    "",
	}
}

func NewWorkspace(id, name string) *mworkspace.Workspace {
	return &mworkspace.Workspace{
		ID:          id,
		Name:        name,
		Description: "desc",
		Prefix:      "TA",
		Theme:       "blue",
	}
}

func NewBoard(wsID, name string, position int, main bool) *mboard.Board {
	return &mboard.Board{ID: NewID(), WorkspaceID: wsID, Name: name, Position: position, IsMain: main}
}

func Must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func MustNotFound(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func MustConflict(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, pkgerrors.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func ErrOf(_ interface{}, err error) error {
	return err
}

var (
	lockMu   sync.Mutex
	lockFile *os.File
)

func serializeTestPackages(t *testing.T) {
	t.Helper()
	lockMu.Lock()
	defer lockMu.Unlock()
	if lockFile != nil {
		return
	}
	f, err := os.OpenFile("/tmp/tandem_repo_test.lock", os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open repo test lock: %v", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		t.Fatalf("lock repo test lock: %v", err)
	}
	lockFile = f
}

var TestCtx = context.Background()
