package requester

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"io"
	"net/http"
	"requester/internal/models"
	"requester/internal/repository"
)

// Processor is a handler for processing tasks.
type Processor interface {
	ProcessTask(ctx context.Context, taskID uuid.UUID) error
	WithLogger(logger *zap.Logger) Processor
}

// processor is a handler for processing tasks.
type processor struct {
	taskRepository repository.TaskRepository
	client         *http.Client
	logger         *zap.Logger
}

// New creates a new processor.
func New(taskRepository repository.TaskRepository, client *http.Client, logger *zap.Logger) (Processor, error) {
	if taskRepository == nil {
		return nil, errors.New("must specify repository.TaskRepository")
	}
	if client == nil {
		return nil, errors.New("must specify *http.Client")
	}
	if logger == nil {
		return nil, errors.New("must specify *zap.Logger")
	}
	return processor{
		taskRepository: taskRepository,
		client:         client,
		logger:         logger,
	}, nil
}

// updateTask updates task.
// Safe to call after task is done.
func (r processor) updateTask(ctx context.Context, task *models.TaskWithResponseData, input *repository.UpdateTaskInput) error {
	if task.Status == models.TaskStatusDone {
		return nil
	}
	input.ID = task.ID
	task.Status = *input.Status
	task.ResponseHeaders = input.ResponseHeaders
	task.ResponseStatusCode = input.ResponseStatusCode
	task.ResponseContentLength = input.ResponseContentLength
	return r.taskRepository.UpdateTask(ctx, input)
}

// makeRequest makes request to a service.
func (r processor) makeRequest(ctx context.Context, task *models.Task) (*http.Response, error) {
	var body io.Reader
	if task.Body != nil {
		data, err := json.Marshal(task.Body)
		if err != nil {
			return nil, nil
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, task.Method, task.URL, body)
	if err != nil {
		return nil, nil
	}

	for k, v := range task.Headers {
		req.Header.Set(k, v)
	}

	return r.client.Do(req)
}

// WithLogger returns a new processor with a new logger.
func (r processor) WithLogger(logger *zap.Logger) Processor {
	return processor{
		taskRepository: r.taskRepository,
		client:         r.client,
		logger:         logger,
	}
}

// ProcessTask processes task.
func (r processor) ProcessTask(ctx context.Context, taskID uuid.UUID) error {
	logg := r.logger.With(zap.String("task_id", taskID.String()))

	task, exists, err := r.taskRepository.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	if !exists {
		logg.Info("task not found")
		return nil
	}

	if task.Status == models.TaskStatusDone {
		logg.Info("task already done")
		return nil
	}
	defer func() {
		err := r.updateTask(ctx, task, &repository.UpdateTaskInput{Status: models.TaskStatusError.Pointer()})
		if err != nil {
			logg.Error("failed to update task status", zap.Error(err))
		}
	}()

	err = r.updateTask(ctx, task, &repository.UpdateTaskInput{
		Status: models.TaskStatusInProcess.Pointer(),
	})
	if err != nil {
		return err
	}

	resp, err := r.makeRequest(ctx, &task.Task)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return r.updateTask(ctx, task, &repository.UpdateTaskInput{
		Status:                models.TaskStatusDone.Pointer(),
		ResponseStatusCode:    &resp.StatusCode,
		ResponseHeaders:       resp.Header,
		ResponseContentLength: &resp.ContentLength,
	})
}
