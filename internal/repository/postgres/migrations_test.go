package postgres_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"jira-issues-go/internal/repository/postgres"
)

func TestMigrationFiles_ReturnsUpFilesInVersionOrder(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"000002_b.up.sql", "000001_a.up.sql", "000001_a.down.sql", "notes.txt"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;"), 0o600))
	}

	files, err := postgres.MigrationFiles(dir)

	require.NoError(t, err)
	require.Len(t, files, 2)
	assert.Equal(t, "000001_a.up.sql", filepath.Base(files[0]))
	assert.Equal(t, "000002_b.up.sql", filepath.Base(files[1]))
}

func TestMigrationFiles_RepositoryMigrationsInOrder(t *testing.T) {
	files, err := postgres.MigrationFiles(filepath.Join("..", "..", "..", "db", "migrations"))

	require.NoError(t, err)
	names := make([]string, len(files))
	for i, file := range files {
		names[i] = filepath.Base(file)
	}
	assert.Equal(t, []string{
		"000001_init_schema.up.sql",
		"000002_issue_parent.up.sql",
		"000003_identity_workspaces.up.sql",
		"000004_work_logs.up.sql",
		"000005_project_members.up.sql",
	}, names)
}

func TestRunMigrations_EmptyDir_FailsBeforeTouchingTheDatabase(t *testing.T) {
	err := postgres.RunMigrations(nil, t.TempDir())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no *.up.sql found")
}
