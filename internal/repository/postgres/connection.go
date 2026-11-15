package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "github.com/lib/pq"
)

// Config holds PostgreSQL connection pool configuration.
type Config struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	ConnectTimeout  time.Duration
}

// DSN builds the PostgreSQL connection string.
func (c Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode,
	)
}

// NewConnection opens and validates a pooled PostgreSQL connection.
func NewConnection(cfg Config) (*sql.DB, error) {
	return OpenDB(cfg.DSN(), cfg.MaxOpenConns, cfg.MaxIdleConns, cfg.ConnMaxLifetime, cfg.ConnMaxIdleTime, cfg.ConnectTimeout)
}

// OpenDB opens a connection from DSN or URL with pool settings.
func OpenDB(dataSourceName string, maxOpen, maxIdle int, maxLifetime, maxIdleTime, timeout time.Duration) (*sql.DB, error) {
	db, err := sql.Open("postgres", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if maxOpen > 0 {
		db.SetMaxOpenConns(maxOpen)
	} else {
		db.SetMaxOpenConns(25)
	}
	if maxIdle > 0 {
		db.SetMaxIdleConns(maxIdle)
	} else {
		db.SetMaxIdleConns(10)
	}
	if maxLifetime > 0 {
		db.SetConnMaxLifetime(maxLifetime)
	} else {
		db.SetConnMaxLifetime(5 * time.Minute)
	}
	if maxIdleTime > 0 {
		db.SetConnMaxIdleTime(maxIdleTime)
	} else {
		db.SetConnMaxIdleTime(1 * time.Minute)
	}

	connectTimeout := timeout
	if connectTimeout <= 0 {
		connectTimeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return db, fmt.Errorf("ping database warning/failure: %w", err)
	}

	return db, nil
}

// MigrationFiles returns the *.up.sql files of migrationsDir in version order (their names start
// with a zero-padded number, so lexical order is version order).
func MigrationFiles(migrationsDir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return nil, fmt.Errorf("failed to read migration files in %s: %w", migrationsDir, err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("failed to read migration files in %s: no *.up.sql found", migrationsDir)
	}
	sort.Strings(files)
	return files, nil
}

// RunMigrations applies every *.up.sql in version order. Each migration is idempotent
// (IF NOT EXISTS), so all of them run on every start-up.
func RunMigrations(db *sql.DB, migrationsDir string) error {
	files, err := MigrationFiles(migrationsDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file, err)
		}
		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", filepath.Base(file), err)
		}
	}
	return nil
}
