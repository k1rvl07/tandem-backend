package attachment

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	"github.com/tandem/tandem/internal/domain/ports/filestore"
	file "github.com/tandem/tandem/internal/usecase/file"
	"github.com/tandem/tandem/internal/usecase/testutil"
	"go.uber.org/zap"
)

type failStore struct {
	files map[string][]byte
}

func newFailStore() *failStore {
	return &failStore{files: make(map[string][]byte)}
}

func (s *failStore) Put(_ context.Context, key string, reader io.Reader, _ int64, _ string) error {
	data, _ := io.ReadAll(reader)
	s.files[key] = data
	return errors.New("put failed")
}

func (s *failStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	data, ok := s.files[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *failStore) Delete(_ context.Context, key string) error {
	delete(s.files, key)
	return nil
}

func (s *failStore) Exists(_ context.Context, key string) (bool, error) {
	_, ok := s.files[key]
	return ok, nil
}

func (s *failStore) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return key, nil
}

func TestCreateRollsBackObjectOnUploadFailure(t *testing.T) {
	attachments := testutil.NewFakeAttachmentRepo()
	tasks := testutil.NewFakeTaskRepo()
	columns := testutil.NewFakeColumnRepo()
	boards := testutil.NewFakeBoardRepo()
	workspaces := testutil.NewFakeWorkspaceRepo()
	hub := testutil.NewFakeHub()
	cache := testutil.NewFakeCache()
	store := newFailStore()

	tasks.AddTaskFixture("t-1", "c-1", "Task", 0)
	tasks.RegisterColumn("c-1", "b-1")
	tasks.RegisterColumnWorkspace("c-1", "ws-1")
	columns.AddColumnFixture("c-1", "b-1", "To do", 0)
	columns.RegisterBoard("ws-1", "b-1")
	boards.AddBoardFixture("b-1", "ws-1", "Board")
	workspaces.AddWorkspaceFixture("ws-1", "WS")
	workspaces.AddMemberFixture("ws-1", "u1", mworkspace.RoleMember)

	svc := NewService(Deps{Attachments: attachments, Tasks: tasks, Columns: columns, Boards: boards, Workspaces: workspaces, Files: file.NewService(store, zap.NewNop()), Hub: hub, Cache: cache})

	_, err := svc.Create(context.Background(), "u1", "ws-1", "t-1", "doc.pdf", "application/pdf", bytes.NewReader([]byte("data")), 4)
	require.Error(t, err)
	assert.Empty(t, store.files)
	assert.Empty(t, attachments.Attachments)
}

var _ filestore.FileStore = (*failStore)(nil)
