package repository

import (
	"context"
	"errors"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/go-faster/jx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"requester/internal/models"
)

// TaskRepository is a repository manager for tasks.
type TaskRepository interface {
	// CreateTask creates a new task.
	CreateTask(ctx context.Context, input *CreateTaskInput) (*models.Task, error)
	// GetTask gets task by id.
	GetTask(ctx context.Context, id uuid.UUID) (_ *models.TaskWithResponseData, exists bool, _ error)
	// UpdateTask updates task.
	UpdateTask(ctx context.Context, input *UpdateTaskInput) error
}

// taskDB is a repository manager for tasks.
type taskDB struct {
	db DBTX
}

// NewTaskDB inits new instance of taskDB.
func NewTaskDB(db DBTX) TaskRepository {
	return taskDB{
		db: db,
	}
}

// BeginTx starts new transaction.
func (q taskDB) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return q.db.Begin(ctx)
}

// WithTx sets transaction.
func (q taskDB) WithTx(tx pgx.Tx) taskDB {
	return taskDB{
		db: tx,
	}
}

// CreateTaskInput is input for CreateTask.
type CreateTaskInput struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    map[string]jx.Raw
}

// setInsertValues sets values for insert query.
func (i *CreateTaskInput) setInsertValues(query sq.InsertBuilder) sq.InsertBuilder {
	columns := []string{"status", "method", "url"}
	values := []interface{}{models.TaskStatusNew, i.Method, i.URL}
	if i.Headers != nil {
		columns = append(columns, "headers")
		values = append(values, i.Headers)
	}
	if i.Body != nil {
		columns = append(columns, "body")
		values = append(values, i.Body)
	}
	return query.Columns(columns...).Values(values...)
}

// CreateTask creates a new task.
func (q taskDB) CreateTask(ctx context.Context, input *CreateTaskInput) (*models.Task, error) {
	if input == nil {
		return nil, fmt.Errorf("input is nil")
	}

	query := sq.Insert("tasks").Suffix("RETURNING id")
	query = input.setInsertValues(query)

	sqlQuery, args, err := query.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, err
	}

	task := &models.Task{
		Status:  models.TaskStatusNew,
		Method:  input.Method,
		URL:     input.URL,
		Headers: input.Headers,
		Body:    input.Body,
	}
	return task, q.db.QueryRow(ctx, sqlQuery, args...).Scan(&task.ID)
}

// GetTask gets task by id.
func (q taskDB) GetTask(ctx context.Context, id uuid.UUID) (_ *models.TaskWithResponseData, exists bool, _ error) {
	query := sq.Select(
		"id",
		"status",
		"method",
		"url",
		"headers",
		"body",
		"response_status_code",
		"response_headers",
		"response_content_length",
	).
		From("tasks").
		Where(sq.Eq{"id": id})

	sqlQuery, args, err := query.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return nil, false, err
	}

	task := &models.TaskWithResponseData{}
	err = q.db.QueryRow(ctx, sqlQuery, args...).Scan(
		&task.ID,
		&task.Status,
		&task.Method,
		&task.URL,
		&task.Headers,
		&task.Body,
		&task.ResponseData.ResponseStatusCode,
		&task.ResponseData.ResponseHeaders,
		&task.ResponseData.ResponseContentLength,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return task, true, nil
}

// UpdateTaskInput is input for UpdateTask.
type UpdateTaskInput struct {
	ID                    uuid.UUID
	Status                *models.TaskStatus
	ResponseStatusCode    *int
	ResponseHeaders       map[string][]string
	ResponseContentLength *int64
}

// setUpdateFields sets fields for update query.
func (i *UpdateTaskInput) setUpdateFields(query sq.UpdateBuilder) sq.UpdateBuilder {
	if i.Status != nil {
		query = query.Set("status", *i.Status)
	}
	if i.ResponseStatusCode != nil {
		query = query.Set("response_status_code", *i.ResponseStatusCode)
	}
	if i.ResponseHeaders != nil {
		query = query.Set("response_headers", i.ResponseHeaders)
	}
	if i.ResponseContentLength != nil {
		query = query.Set("response_content_length", *i.ResponseContentLength)
	}
	return query
}

// UpdateTask updates task.
func (q taskDB) UpdateTask(ctx context.Context, input *UpdateTaskInput) error {
	if input == nil || input.ID == uuid.Nil {
		return fmt.Errorf("input is nil or id is empty")
	}

	query := sq.Update("tasks").Where(sq.Eq{"id": input.ID})
	query = input.setUpdateFields(query)

	sqlQuery, args, err := query.PlaceholderFormat(sq.Dollar).ToSql()
	if err != nil {
		return err
	}

	_, err = q.db.Exec(ctx, sqlQuery, args...)
	if err != nil {
		return err
	}
	return nil
}
