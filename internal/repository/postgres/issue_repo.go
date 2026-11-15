package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/repository"
)

// issueColumns is the column list read by every query; scanIssue reads them in this order.
const issueColumns = `id, key, project_id, title, description, status, priority,
		       type, story_points, sprint_id, assignee_id, reporter_id,
		       acceptance_criteria, version, created_at, updated_at, parent_id`

// rowScanner is implemented by *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

type issueRepo struct {
	db *sql.DB
}

// NewIssueRepository constructs a new PostgreSQL IssueRepository.
func NewIssueRepository(db *sql.DB) repository.IssueRepository {
	return &issueRepo{db: db}
}

func (r *issueRepo) GetNextSequence(ctx context.Context, projectID string) (int64, string, error) {
	query := `
		UPDATE projects
		SET issue_counter = issue_counter + 1, updated_at = NOW()
		WHERE id = $1
		RETURNING issue_counter, key;
	`
	var counter int64
	var keyPrefix string
	err := r.db.QueryRowContext(ctx, query, projectID).Scan(&counter, &keyPrefix)
	if err != nil {
		return 0, "", MapPgError(err)
	}
	return counter, keyPrefix, nil
}

// nullableString maps nil or blank values to SQL NULL.
func nullableString(value *string) sql.NullString {
	if value == nil || *value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

func (r *issueRepo) Create(ctx context.Context, issue *domain.Issue) error {
	if issue.ID == "" {
		issue.ID = uuid.New().String()
	}

	if issue.Key == "" {
		seq, prefix, err := r.GetNextSequence(ctx, issue.ProjectID)
		if err == nil && prefix != "" {
			issue.Key = fmt.Sprintf("%s-%d", prefix, seq)
		} else {
			issue.Key = fmt.Sprintf("PROJ-%d", time.Now().UnixNano()%100000)
		}
	}

	query := `
		INSERT INTO issues (
			id, key, project_id, title, description, status, priority,
			type, story_points, sprint_id, assignee_id, reporter_id,
			acceptance_criteria, version, created_at, updated_at, parent_id
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
		)
		RETURNING created_at, updated_at;
	`

	now := time.Now().UTC()
	if issue.CreatedAt.IsZero() {
		issue.CreatedAt = now
	}
	if issue.UpdatedAt.IsZero() {
		issue.UpdatedAt = now
	}
	if issue.Version == 0 {
		issue.Version = 1
	}

	err := r.db.QueryRowContext(
		ctx, query,
		issue.ID,
		issue.Key,
		issue.ProjectID,
		issue.Title,
		issue.Description,
		issue.Status.String(),
		issue.Priority.String(),
		issue.Type.String(),
		issue.StoryPoints,
		nullableString(issue.SprintID),
		nullableString(issue.AssigneeID),
		nullableString(issue.ReporterID),
		issue.AcceptanceCriteria,
		issue.Version,
		issue.CreatedAt,
		issue.UpdatedAt,
		nullableString(issue.ParentID),
	).Scan(&issue.CreatedAt, &issue.UpdatedAt)

	return MapPgError(err)
}

func (r *issueRepo) GetByID(ctx context.Context, id string) (*domain.Issue, error) {
	query := `SELECT ` + issueColumns + ` FROM issues WHERE id = $1;`
	return scanIssue(r.db.QueryRowContext(ctx, query, id))
}

func (r *issueRepo) GetByKey(ctx context.Context, key string) (*domain.Issue, error) {
	query := `SELECT ` + issueColumns + ` FROM issues WHERE key = $1;`
	return scanIssue(r.db.QueryRowContext(ctx, query, key))
}

func (r *issueRepo) GetByIDOrKey(ctx context.Context, idOrKey string) (*domain.Issue, error) {
	// First check by ID
	issue, err := r.GetByID(ctx, idOrKey)
	if err == nil && issue != nil {
		return issue, nil
	}
	// Fallback to check by Key
	return r.GetByKey(ctx, idOrKey)
}

func (r *issueRepo) List(ctx context.Context, filter repository.ListFilter) ([]*domain.Issue, int32, error) {
	var conditions []string
	var args []interface{}
	idx := 1

	if filter.ProjectID != "" {
		conditions = append(conditions, fmt.Sprintf("project_id = $%d", idx))
		args = append(args, filter.ProjectID)
		idx++
	}
	if filter.SprintID != nil && *filter.SprintID != "" {
		conditions = append(conditions, fmt.Sprintf("sprint_id = $%d", idx))
		args = append(args, *filter.SprintID)
		idx++
	}
	if filter.Status != domain.StatusUnspecified {
		conditions = append(conditions, fmt.Sprintf("status = $%d", idx))
		args = append(args, filter.Status.String())
		idx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
		SELECT %s,
		       COUNT(*) OVER() AS total_count
		FROM issues
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d;
	`, issueColumns, whereClause, idx, idx+1)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, MapPgError(err)
	}
	defer rows.Close()

	var issues []*domain.Issue
	var totalCount int32

	for rows.Next() {
		var tc int32
		issue, err := scanIssue(rows, &tc)
		if err != nil {
			return nil, 0, err
		}
		totalCount = tc
		issues = append(issues, issue)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, MapPgError(err)
	}

	return issues, totalCount, nil
}

// UpdateStatus changes only the status column, guarded by the version that was read. It never
// rewrites other columns, so a concurrent sprint assignment cannot be undone by a status change.
func (r *issueRepo) UpdateStatus(ctx context.Context, issue *domain.Issue, expectedVersion int) error {
	const query = `
		UPDATE issues
		SET status = $1, version = version + 1, updated_at = NOW()
		WHERE id = $2 AND version = $3
		RETURNING version, updated_at;
	`
	err := r.db.QueryRowContext(ctx, query, issue.Status.String(), issue.ID, expectedVersion).
		Scan(&issue.Version, &issue.UpdatedAt)
	if !errors.Is(err, sql.ErrNoRows) {
		return MapPgError(err)
	}

	// No row matched: either the issue is gone or its version moved on.
	var exists bool
	if err := r.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM issues WHERE id = $1)`, issue.ID).Scan(&exists); err != nil {
		return MapPgError(err)
	}
	if !exists {
		return domain.ErrNotFound
	}
	return domain.ErrVersionConflict
}

func (r *issueRepo) BatchUpdateSprint(ctx context.Context, ids []string, sprintID string) ([]*domain.Issue, error) {
	if len(ids) == 0 {
		return []*domain.Issue{}, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, MapPgError(err)
	}
	defer tx.Rollback()

	var sprintIDNull sql.NullString
	if strings.TrimSpace(sprintID) != "" {
		sprintIDNull = sql.NullString{String: strings.TrimSpace(sprintID), Valid: true}
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids)+1)
	args[0] = sprintIDNull
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args[i+1] = id
	}

	query := fmt.Sprintf(`
		UPDATE issues
		SET sprint_id = $1, updated_at = NOW(), version = version + 1
		WHERE id IN (%s)
		RETURNING %s;
	`, strings.Join(placeholders, ", "), issueColumns)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, MapPgError(err)
	}
	defer rows.Close()

	var updated []*domain.Issue
	for rows.Next() {
		issue, err := scanIssue(rows)
		if err != nil {
			return nil, err
		}
		updated = append(updated, issue)
	}

	if err := rows.Err(); err != nil {
		return nil, MapPgError(err)
	}

	if err := tx.Commit(); err != nil {
		return nil, MapPgError(err)
	}

	return updated, nil
}

// scanIssue reads one row selected with issueColumns, followed by any extra columns.
func scanIssue(row rowScanner, extra ...any) (*domain.Issue, error) {
	var issue domain.Issue
	var statusStr, priorityStr, typeStr string
	var sprintNull, assigneeNull, reporterNull, parentNull sql.NullString

	dest := []any{
		&issue.ID,
		&issue.Key,
		&issue.ProjectID,
		&issue.Title,
		&issue.Description,
		&statusStr,
		&priorityStr,
		&typeStr,
		&issue.StoryPoints,
		&sprintNull,
		&assigneeNull,
		&reporterNull,
		&issue.AcceptanceCriteria,
		&issue.Version,
		&issue.CreatedAt,
		&issue.UpdatedAt,
		&parentNull,
	}
	if err := row.Scan(append(dest, extra...)...); err != nil {
		return nil, MapPgError(err)
	}

	issue.Status = domain.ParseStatus(statusStr)
	issue.Priority = domain.ParsePriority(priorityStr)
	issue.Type = domain.ParseType(typeStr)
	issue.SprintID = stringPtr(sprintNull)
	issue.AssigneeID = stringPtr(assigneeNull)
	issue.ReporterID = stringPtr(reporterNull)
	issue.ParentID = stringPtr(parentNull)

	return &issue, nil
}

func stringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}
