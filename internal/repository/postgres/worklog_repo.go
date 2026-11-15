package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/repository"
)

// workLogSelect reads an entry with its issue and user; scanWorkLog reads the columns in this order.
const workLogSelect = `SELECT wl.id, wl.project_id, wl.issue_id, i.key, i.title, wl.user_id, u.name,
       wl.sprint_id, wl.hours::float8, wl.note, wl.logged_at`

type workLogRepo struct {
	db *sql.DB
}

// NewWorkLogRepository constructs the PostgreSQL WorkLogRepository (migration 000004).
func NewWorkLogRepository(db *sql.DB) repository.WorkLogRepository {
	return &workLogRepo{db: db}
}

func scanWorkLog(row rowScanner) (*domain.WorkLog, error) {
	var log domain.WorkLog
	var sprintID sql.NullString
	if err := row.Scan(&log.ID, &log.ProjectID, &log.IssueID, &log.IssueKey, &log.IssueTitle, &log.UserID,
		&log.UserName, &sprintID, &log.Hours, &log.Note, &log.LoggedAt); err != nil {
		return nil, err
	}
	log.SprintID = sprintID.String
	return &log, nil
}

func (r *workLogRepo) Create(ctx context.Context, log *domain.WorkLog) error {
	row := r.db.QueryRowContext(ctx, `
		WITH wl AS (
			INSERT INTO work_logs (project_id, issue_id, user_id, sprint_id, hours, note)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING *
		)
		`+workLogSelect+`
		  FROM wl
		  JOIN issues i ON i.id = wl.issue_id
		  JOIN app_users u ON u.id = wl.user_id`,
		log.ProjectID, log.IssueID, log.UserID, nullableString(&log.SprintID), log.Hours, log.Note)
	created, err := scanWorkLog(row)
	if err != nil {
		return fmt.Errorf("insert work log: %w", MapPgError(err))
	}
	*log = *created
	return nil
}

func (r *workLogRepo) ListByProject(ctx context.Context, projectID string, limit int) ([]*domain.WorkLog, error) {
	rows, err := r.db.QueryContext(ctx, workLogSelect+`
		  FROM work_logs wl
		  JOIN issues i ON i.id = wl.issue_id
		  JOIN app_users u ON u.id = wl.user_id
		 WHERE wl.project_id = $1
		 ORDER BY wl.logged_at DESC, wl.id
		 LIMIT $2`, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("list work logs: %w", err)
	}
	defer rows.Close()

	logs := []*domain.WorkLog{}
	for rows.Next() {
		log, err := scanWorkLog(rows)
		if err != nil {
			return nil, fmt.Errorf("scan work log: %w", err)
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

func (r *workLogRepo) sum(ctx context.Context, query string, args ...any) (float64, error) {
	var total float64
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("sum work log hours: %w", err)
	}
	return total, nil
}

func (r *workLogRepo) SumProjectHours(ctx context.Context, projectID string) (float64, error) {
	return r.sum(ctx, `SELECT COALESCE(SUM(hours), 0)::float8 FROM work_logs WHERE project_id = $1`, projectID)
}

func (r *workLogRepo) SumSprintHours(ctx context.Context, projectID, sprintID string) (float64, error) {
	return r.sum(ctx, `SELECT COALESCE(SUM(hours), 0)::float8 FROM work_logs WHERE project_id = $1 AND sprint_id = $2`,
		projectID, sprintID)
}
