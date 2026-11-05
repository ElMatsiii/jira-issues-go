package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
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

// RunMigrations executes SQL migrations from a file on disk.
func RunMigrations(db *sql.DB, migrationsDir string) error {
	upFile := filepath.Join(migrationsDir, "000001_init_schema.up.sql")
	content, err := os.ReadFile(upFile)
	if err != nil {
		return fmt.Errorf("failed to read migration file %s: %w", upFile, err)
	}

	if _, err := db.Exec(string(content)); err != nil {
		return fmt.Errorf("failed to apply migration: %w", err)
	}
	return nil
}
