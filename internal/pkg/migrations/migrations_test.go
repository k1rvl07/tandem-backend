package migrations_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tandem/tandem/internal/pkg/migrations"
	"github.com/tandem/tandem/internal/repository/testutil"
)

func TestRunIsIdempotent(t *testing.T) {
	db := testutil.NewTestDB(t)

	require.NoError(t, migrations.Run(db))

	var count int64
	require.NoError(t, db.Raw("SELECT count(*) FROM schema_migrations").Scan(&count).Error)
	require.Greater(t, count, int64(0), "schema_migrations should have rows")
}

func TestRunTwiceNoError(t *testing.T) {
	db := testutil.NewTestDB(t)

	require.NoError(t, migrations.Run(db))

	var count1 int64
	require.NoError(t, db.Raw("SELECT count(*) FROM schema_migrations").Scan(&count1).Error)

	require.NoError(t, migrations.Run(db))

	var count2 int64
	require.NoError(t, db.Raw("SELECT count(*) FROM schema_migrations").Scan(&count2).Error)
	require.Equal(t, count1, count2, "second Run should not insert new rows")
}

func TestSchemaMigrationsVersionsAreSQLFiles(t *testing.T) {
	db := testutil.NewTestDB(t)

	var versions []string
	require.NoError(t, db.Raw("SELECT version FROM schema_migrations ORDER BY version").Scan(&versions).Error)

	require.Equal(t, []string{
		"0001_legacy_task_id_columns.sql",
		"0002_drop_task_comments.sql",
		"0003_workspace_viewer_to_member.sql",
	}, versions)
}
