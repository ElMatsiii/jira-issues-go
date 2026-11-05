package postgres

import (
	"database/sql"
	"errors"

	"github.com/lib/pq"
	"jira-issues-go/internal/domain"
)

var (
	ErrDuplicateKey   = errors.New("unique constraint violation")
	ErrForeignKeyFail = errors.New("foreign key constraint violation")
	ErrInvalidInput   = errors.New("invalid repository input")
)

// MapPgError translates database/pq errors into domain or known repository errors.
func MapPgError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}

	var pgErr *pq.Error
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return ErrDuplicateKey
		case "23503": // foreign_key_violation
			return ErrForeignKeyFail
		}
	}
	return err
}
