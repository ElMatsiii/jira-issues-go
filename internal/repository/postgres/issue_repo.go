package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"jira-issues-go/internal/domain"
	"jira-issues-go/internal/repository"
)

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
			acceptance_criteria, version, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
		)
		RETURNING created_at, updated_at;
	`

	var sprintIDNull sql.NullString
	if issue.SprintID != nil && *issue.SprintID != "" {
		sprintIDNull = sql.NullString{String: *issue.SprintID, Valid: true}
	}
	var assigneeNull sql.NullString
	if issue.AssigneeID != nil && *issue.AssigneeID != "" {
		assigneeNull = sql.NullString{String: *issue.AssigneeID, Valid: true}
	}
	var reporterNull sql.NullString
	if issue.ReporterID != nil && *issue.ReporterID != "" {
		reporterNull = sql.NullString{String: *issue.ReporterID, Valid: true}
	}

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
		sprintIDNull,
		assigneeNull,
		reporterNull,
		issue.AcceptanceCriteria,
		issue.Version,
		issue.CreatedAt,
		issue.UpdatedAt,
	).Scan(&issue.CreatedAt, &issue.UpdatedAt)

	return MapPgError(err)
}

func (r *issueRepo) GetByID(ctx context.Context, id string) (*domain.Issue, error) {
	query := `
		SELECT id, key, project_id, title, description, status, priority,
		       type, story_points, sprint_id, assignee_id, reporter_id,
		       acceptance_criteria, version, created_at, updated_at
		FROM issues
		WHERE id = $1;
	`
	return r.scanRow(r.db.QueryRowContext(ctx, query, id))
}

func (r *issueRepo) GetByKey(ctx context.Context, key string) (*domain.Issue, error) {
	query := `
		SELECT id, key, project_id, title, description, status, priority,
		       type, story_points, sprint_id, assignee_id, reporter_id,
		       acceptance_criteria, version, created_at, updated_at
		FROM issues
		WHERE key = $1;
	`
	return r.scanRow(r.db.QueryRowContext(ctx, query, key))
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
		SELECT id, key, project_id, title, description, status, priority,
		       type, story_points, sprint_id, assignee_id, reporter_id,
		       acceptance_criteria, version, created_at, updated_at,
		       COUNT(*) OVER() AS total_count
		FROM issues
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d;
	`, whereClause, idx, idx+1)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, MapPgError(err)
	}
	defer rows.Close()

	var issues []*domain.Issue
	var totalCount int32

	for rows.Next() {
		var issue domain.Issue
		var statusStr, priorityStr, typeStr string
		var sprintNull, assigneeNull, reporterNull sql.NullString
		var tc int32

		err := rows.Scan(
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
			&tc,
		)
		if err != nil {
			return nil, 0, MapPgError(err)
		}

		issue.Status = domain.ParseStatus(statusStr)
		issue.Priority = domain.ParsePriority(priorityStr)
		issue.Type = domain.ParseType(typeStr)
		if sprintNull.Valid {
			issue.SprintID = &sprintNull.String
		}
		if assigneeNull.Valid {
			issue.AssigneeID = &assigneeNull.String
		}
		if reporterNull.Valid {
			issue.ReporterID = &reporterNull.String
		}

		totalCount = tc
		issues = append(issues, &issue)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, MapPgError(err)
	}

	return issues, totalCount, nil
}

func (r *issueRepo) Update(ctx context.Context, issue *domain.Issue) error {
	query := `
		UPDATE issues
		SET title = $1, description = $2, status = $3, priority = $4,
		    type = $5, story_points = $6, sprint_id = $7, assignee_id = $8,
		    reporter_id = $9, acceptance_criteria = $10, version = version + 1,
		    updated_at = NOW()
		WHERE id = $11
		RETURNING version, updated_at;
	`
	var sprintIDNull, assigneeNull, reporterNull sql.NullString
	if issue.SprintID != nil && *issue.SprintID != "" {
		sprintIDNull = sql.NullString{String: *issue.SprintID, Valid: true}
	}
	if issue.AssigneeID != nil && *issue.AssigneeID != "" {
		assigneeNull = sql.NullString{String: *issue.AssigneeID, Valid: true}
	}
	if issue.ReporterID != nil && *issue.ReporterID != "" {
		reporterNull = sql.NullString{String: *issue.ReporterID, Valid: true}
	}

	err := r.db.QueryRowContext(
		ctx, query,
		issue.Title,
		issue.Description,
		issue.Status.String(),
		issue.Priority.String(),
		issue.Type.String(),
		issue.StoryPoints,
		sprintIDNull,
		assigneeNull,
		reporterNull,
		issue.AcceptanceCriteria,
		issue.ID,
	).Scan(&issue.Version, &issue.UpdatedAt)

	return MapPgError(err)
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
		RETURNING id, key, project_id, title, description, status, priority,
		          type, story_points, sprint_id, assignee_id, reporter_id,
		          acceptance_criteria, version, created_at, updated_at;
	`, strings.Join(placeholders, ", "))

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, MapPgError(err)
	}
	defer rows.Close()

	var updated []*domain.Issue
	for rows.Next() {
		var issue domain.Issue
		var statusStr, priorityStr, typeStr string
		var sNull, aNull, rNull sql.NullString

		if err := rows.Scan(
			&issue.ID,
			&issue.Key,
			&issue.ProjectID,
			&issue.Title,
			&issue.Description,
			&statusStr,
			&priorityStr,
			&typeStr,
			&issue.StoryPoints,
			&sNull,
			&aNull,
			&rNull,
			&issue.AcceptanceCriteria,
			&issue.Version,
			&issue.CreatedAt,
			&issue.UpdatedAt,
		); err != nil {
			return nil, MapPgError(err)
		}

		issue.Status = domain.ParseStatus(statusStr)
		issue.Priority = domain.ParsePriority(priorityStr)
		issue.Type = domain.ParseType(typeStr)
		if sNull.Valid {
			issue.SprintID = &sNull.String
		}
		if aNull.Valid {
			issue.AssigneeID = &aNull.String
		}
		if rNull.Valid {
			issue.ReporterID = &rNull.String
		}
		updated = append(updated, &issue)
	}

	if err := rows.Err(); err != nil {
		return nil, MapPgError(err)
	}

	if err := tx.Commit(); err != nil {
		return nil, MapPgError(err)
	}

	return updated, nil
}

func (r *issueRepo) scanRow(row *sql.Row) (*domain.Issue, error) {
	var issue domain.Issue
	var statusStr, priorityStr, typeStr string
	var sprintNull, assigneeNull, reporterNull sql.NullString

	err := row.Scan(
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
	)
	if err != nil {
		return nil, MapPgError(err)
	}

	issue.Status = domain.ParseStatus(statusStr)
	issue.Priority = domain.ParsePriority(priorityStr)
	issue.Type = domain.ParseType(typeStr)
	if sprintNull.Valid {
		issue.SprintID = &sprintNull.String
	}
	if assigneeNull.Valid {
		issue.AssigneeID = &assigneeNull.String
	}
	if reporterNull.Valid {
		issue.ReporterID = &reporterNull.String
	}

	return &issue, nil
}
