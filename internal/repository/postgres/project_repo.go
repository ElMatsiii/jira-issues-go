package postgres

import (
	"context"
	"database/sql"

	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/repository"
)

type projectRepo struct {
	db *sql.DB
}

// NewProjectRepository constructs a new PostgreSQL ProjectRepository.
func NewProjectRepository(db *sql.DB) repository.ProjectRepository {
	return &projectRepo{db: db}
}

func (r *projectRepo) Create(ctx context.Context, project *domain.Project) error {
	query := `
		INSERT INTO projects (id, key, name, description, issue_counter, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at;
	`
	err := r.db.QueryRowContext(
		ctx, query,
		project.ID,
		project.Key,
		project.Name,
		project.Description,
		project.IssueCounter,
		project.CreatedAt,
		project.UpdatedAt,
	).Scan(&project.CreatedAt, &project.UpdatedAt)

	return MapPgError(err)
}

func (r *projectRepo) GetByID(ctx context.Context, id string) (*domain.Project, error) {
	query := `SELECT id, key, name, description, issue_counter, created_at, updated_at FROM projects WHERE id = $1;`
	var p domain.Project
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&p.ID, &p.Key, &p.Name, &p.Description, &p.IssueCounter, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, MapPgError(err)
	}
	return &p, nil
}

func (r *projectRepo) GetByKey(ctx context.Context, key string) (*domain.Project, error) {
	query := `SELECT id, key, name, description, issue_counter, created_at, updated_at FROM projects WHERE key = $1;`
	var p domain.Project
	err := r.db.QueryRowContext(ctx, query, key).Scan(
		&p.ID, &p.Key, &p.Name, &p.Description, &p.IssueCounter, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, MapPgError(err)
	}
	return &p, nil
}
