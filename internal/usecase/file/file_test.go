package file

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type memStore struct {
	files   map[string][]byte
	deleted []string
}

func newMemStore() *memStore {
	return &memStore{files: make(map[string][]byte)}
}

func (s *memStore) Put(_ context.Context, key string, reader io.Reader, _ int64, _ string) error {
	data, _ := io.ReadAll(reader)
	s.files[key] = data
	return nil
}

func (s *memStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	data, ok := s.files[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *memStore) Delete(_ context.Context, key string) error {
	delete(s.files, key)
	s.deleted = append(s.deleted, key)
	return nil
}

func (s *memStore) Exists(_ context.Context, key string) (bool, error) {
	_, ok := s.files[key]
	return ok, nil
}

func (s *memStore) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return key, nil
}

func TestDeleteImageOwnedByActor(t *testing.T) {
	store := newMemStore()
	svc := NewService(store, zap.NewNop())
	key := "covers/u1/" + uuid.NewString() + ".jpg"
	store.files[key] = []byte("img")

	err := svc.DeleteImage(context.Background(), "u1", key)
	require.NoError(t, err)
	assert.Equal(t, []string{key}, store.deleted)
}

func TestDeleteImageForbiddenOwner(t *testing.T) {
	store := newMemStore()
	svc := NewService(store, zap.NewNop())
	key := "covers/u2/" + uuid.NewString() + ".jpg"

	err := svc.DeleteImage(context.Background(), "u1", key)
	require.Error(t, err)
	assert.Empty(t, store.deleted)
}

func TestDeleteImageRejectsAttachmentKey(t *testing.T) {
	store := newMemStore()
	svc := NewService(store, zap.NewNop())

	err := svc.DeleteImage(context.Background(), "u1", "attachments/ws1/u1/file.pdf")
	require.Error(t, err)
	assert.Empty(t, store.deleted)
}
