package miniofs

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/require"
	"github.com/tandem/tandem/internal/pkg/config"
)

func newMinio(t *testing.T) (*MinIO, *httptest.Server) {
	t.Helper()
	fake := gofakes3.New(s3mem.New())
	srv := httptest.NewServer(fake.Server())
	t.Cleanup(srv.Close)

	fs, err := New(config.MinIOConfig{
		Endpoint:  srv.URL,
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "tandem-files",
	})
	require.NoError(t, err)
	return fs, srv
}

func TestNewCreatesBucket(t *testing.T) {
	fs, _ := newMinio(t)
	require.Equal(t, "tandem-files", fs.Bucket())
}

func TestNewExistingBucketReused(t *testing.T) {
	fs, srv := newMinio(t)

	fs2, err := New(config.MinIOConfig{
		Endpoint:  srv.URL,
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "tandem-files",
	})
	require.NoError(t, err)
	exists, err := fs2.Exists(context.Background(), "missing-key")
	require.NoError(t, err)
	require.False(t, exists)
	_ = fs
}

func TestNewUnreachableEndpoint(t *testing.T) {
	_, err := New(config.MinIOConfig{
		Endpoint:  "http://127.0.0.1:1",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "tandem-files",
	})
	require.Error(t, err)
}

func TestNewInvalidPublicEndpoint(t *testing.T) {
	_, err := New(config.MinIOConfig{
		Endpoint:       "http://127.0.0.1:1",
		AccessKey:      "minioadmin",
		SecretKey:      "minioadmin",
		Bucket:         "tandem-files",
		PublicEndpoint: "://bad",
	})
	require.Error(t, err)
}

func TestNewInvalidPrimaryEndpoint(t *testing.T) {
	_, err := New(config.MinIOConfig{
		Endpoint:  "://bad",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "tandem-files",
	})
	require.Error(t, err)
}

func TestNewWithPublicEndpoint(t *testing.T) {
	fake := gofakes3.New(s3mem.New())
	srv := httptest.NewServer(fake.Server())
	defer srv.Close()

	fs, err := New(config.MinIOConfig{
		Endpoint:       srv.URL,
		PublicEndpoint: srv.URL,
		AccessKey:      "minioadmin",
		SecretKey:      "minioadmin",
		Bucket:         "tandem-files",
	})
	require.NoError(t, err)
	require.NotNil(t, fs.public)
	require.NotNil(t, fs.client)
}

func TestNewMakeBucketError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		switch {
		case r.Method == "GET":
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, `<Error><Code>NoSuchBucket</Code><Message>missing</Message></Error>`)
		case r.Method == "PUT":
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `<Error><Code>InvalidBucketName</Code><Message>bad name</Message></Error>`)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	_, err := New(config.MinIOConfig{
		Endpoint:  srv.URL,
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "tandem-files",
	})
	require.Error(t, err)
}

func brokenMinio(t *testing.T) *MinIO {
	t.Helper()
	c, err := minio.New("http://127.0.0.1:1", &minio.Options{
		Creds: credentials.NewStaticV4("a", "b", ""),
	})
	require.NoError(t, err)
	return &MinIO{client: c, public: c, bucket: "mybucket"}
}

func TestGetInvalidBucket(t *testing.T) {
	c, err := minio.New("http://127.0.0.1:1", &minio.Options{
		Creds: credentials.NewStaticV4("a", "b", ""),
	})
	require.NoError(t, err)
	fs := &MinIO{client: c, public: c, bucket: "ab"}
	_, err = fs.Get(context.Background(), "key")
	require.Error(t, err)
}

func TestExistsNonNoSuchKeyError(t *testing.T) {
	exists, err := brokenMinio(t).Exists(context.Background(), "key")
	require.Error(t, err)
	require.False(t, exists)
}

func TestDeleteBrokenClient(t *testing.T) {
	require.Error(t, brokenMinio(t).Delete(context.Background(), "key"))
}

func TestPresignGetError(t *testing.T) {
	_, err := brokenMinio(t).PresignGet(context.Background(), "key", time.Hour)
	require.Error(t, err)
}

func TestPutGetDeleteRoundtrip(t *testing.T) {
	fs, _ := newMinio(t)
	ctx := context.Background()
	key := "images/user-1/uuid1.png"

	require.NoError(t, fs.Put(ctx, key, strings.NewReader("payload"), 7, "image/png"))

	exists, err := fs.Exists(ctx, key)
	require.NoError(t, err)
	require.True(t, exists)

	rc, err := fs.Get(ctx, key)
	require.NoError(t, err)
	defer rc.Close()
	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, "payload", string(data))

	require.NoError(t, fs.Delete(ctx, key))

	exists, err = fs.Exists(ctx, key)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestPutOverwrite(t *testing.T) {
	fs, _ := newMinio(t)
	ctx := context.Background()
	key := "images/user-1/uuid2.png"

	require.NoError(t, fs.Put(ctx, key, strings.NewReader("first"), 5, "text/plain"))
	require.NoError(t, fs.Put(ctx, key, strings.NewReader("second-longer"), 13, "text/plain"))

	rc, err := fs.Get(ctx, key)
	require.NoError(t, err)
	defer rc.Close()
	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, "second-longer", string(data))
}

func TestPutDefaultContentType(t *testing.T) {
	fs, _ := newMinio(t)
	ctx := context.Background()
	key := "attachments/user-1/uuid3.bin"

	require.NoError(t, fs.Put(ctx, key, strings.NewReader("x"), 1, ""))

	obj, err := fs.client.GetObject(ctx, fs.bucket, key, minio.GetObjectOptions{})
	require.NoError(t, err)
	info, err := obj.Stat()
	require.NoError(t, err)
	require.Equal(t, "application/octet-stream", info.ContentType)
	_ = obj.Close()
}

func TestGetMissingKey(t *testing.T) {
	fs, _ := newMinio(t)
	ctx := context.Background()

	rc, err := fs.Get(ctx, "no/such/key")
	require.NoError(t, err)
	defer rc.Close()
	_, err = io.ReadAll(rc)
	require.Error(t, err)
}

func TestExistsMissingKey(t *testing.T) {
	fs, _ := newMinio(t)
	exists, err := fs.Exists(context.Background(), "no/such/key")
	require.NoError(t, err)
	require.False(t, exists)
}

func TestDeleteMissingKey(t *testing.T) {
	fs, _ := newMinio(t)
	require.NoError(t, fs.Delete(context.Background(), "no/such/key"))
}

func TestPresignGet(t *testing.T) {
	fs, _ := newMinio(t)
	ctx := context.Background()
	key := "avatars/user-1/uuid4.png"
	require.NoError(t, fs.Put(ctx, key, strings.NewReader("avatar"), 6, "image/png"))

	url, err := fs.PresignGet(ctx, key, time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, url)
	require.Contains(t, url, "tandem-files")
	require.Contains(t, url, key)

	url2, err := fs.PresignGet(ctx, key, time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, url2)
	require.NotEqual(t, url, url2)
}
