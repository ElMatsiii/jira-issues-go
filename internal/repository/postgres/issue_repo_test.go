package postgres_test

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/repository/postgres"
)

func TestPostgres_Config_DSN(t *testing.T) {
	cfg := postgres.Config{
		Host:            "db.example.com",
		Port:            5432,
		User:            "jira_user",
		Password:        "secret",
		Database:        "issues_db",
		SSLMode:         "disable",
		MaxOpenConns:    20,
		MaxIdleConns:    5,
		ConnMaxLifetime: 10 * time.Minute,
		ConnMaxIdleTime: 2 * time.Minute,
		ConnectTimeout:  3 * time.Second,
	}

	dsn := cfg.DSN()
	assert.Contains(t, dsn, "host=db.example.com")
	assert.Contains(t, dsn, "port=5432")
	assert.Contains(t, dsn, "user=jira_user")
	assert.Contains(t, dsn, "password=secret")
	assert.Contains(t, dsn, "dbname=issues_db")
	assert.Contains(t, dsn, "sslmode=disable")
}

func TestPostgres_MapPgError(t *testing.T) {
	// Nil error
	assert.NoError(t, postgres.MapPgError(nil))

	// ErrNoRows -> domain.ErrNotFound
	err := postgres.MapPgError(sql.ErrNoRows)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	// pq unique violation "23505"
	pqUnique := &pq.Error{Code: "23505", Message: "duplicate key"}
	err = postgres.MapPgError(pqUnique)
	assert.ErrorIs(t, err, postgres.ErrDuplicateKey)

	// pq foreign key violation "23503"
	pqFk := &pq.Error{Code: "23503", Message: "foreign key fail"}
	err = postgres.MapPgError(pqFk)
	assert.ErrorIs(t, err, postgres.ErrForeignKeyFail)

	// Generic error preserved
	genericErr := errors.New("custom connection failure")
	err = postgres.MapPgError(genericErr)
	assert.Equal(t, genericErr, err)
}

func TestPostgres_RunMigrations_InvalidDir(t *testing.T) {
	err := postgres.RunMigrations(nil, "non_existent_directory_xyz")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read migration file")
}

func TestPostgres_OpenDB_InvalidHost(t *testing.T) {
	// Connecting to invalid host should return warning/failure from PingContext
	db, err := postgres.OpenDB("postgres://invalid:invalid@127.0.0.1:59999/issues_db?sslmode=disable", 1, 1, time.Second, time.Second, 100*time.Millisecond)
	if err != nil {
		assert.Contains(t, err.Error(), "ping database warning/failure")
	}
	if db != nil {
		_ = db.Close()
	}
}

func TestPostgres_Constructors(t *testing.T) {
	repo := postgres.NewIssueRepository(nil)
	assert.NotNil(t, repo)

	pRepo := postgres.NewProjectRepository(nil)
	assert.NotNil(t, pRepo)

	cfg := postgres.Config{
		Host:           "127.0.0.1",
		Port:           59998,
		ConnectTimeout: 50 * time.Millisecond,
	}
	db, err := postgres.NewConnection(cfg)
	if err != nil {
		assert.Contains(t, err.Error(), "ping database warning/failure")
	}
	if db != nil {
		_ = db.Close()
	}
}
