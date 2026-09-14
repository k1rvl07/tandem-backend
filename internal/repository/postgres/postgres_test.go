package postgres

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tandem/tandem/internal/pkg/config"
)

func TestDSNFormatting(t *testing.T) {
	cfg := config.DatabaseConfig{Host: "h", Port: "5432", User: "u", Password: "p", Name: "n"}
	require.Equal(t, "host=h port=5432 user=u password=p dbname=n sslmode=disable", dsn(cfg))
}

func TestIsUniqueViolation(t *testing.T) {
	require.False(t, IsUniqueViolation(nil))
	require.False(t, IsUniqueViolation(errors.New("plain error")))
	require.True(t, IsUniqueViolation(errors.New("duplicate key value violates unique constraint \"users_pkey\"")))
	require.True(t, IsUniqueViolation(errors.New("UNIQUE constraint failed: users.login")))
}

func TestWithConnectTimeout(t *testing.T) {
	o := options{}
	opt := WithConnectTimeout(42 * time.Second)
	opt(&o)
	require.Equal(t, 42*time.Second, o.ConnectTimeout)
}
