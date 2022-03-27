package api

import (
	"context"
	"fmt"
	"requester/internal/api/oas"
	"requester/internal/models"
	"requester/internal/repository"
	"time"
)

// CreateTask creates new task.
func (h *handler) CreateTask(ctx context.Context, req *oas.CreateTaskInput) (*oas.CreateTaskOutput, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	task, err := h.taskRepository.CreateTask(ctx, &repository.CreateTaskInput{
		Method:  string(req.Method),
		URL:     req.URL,
		Headers: req.Headers.Value,
		Body:    req.Body.Value,
	})
	if err != nil {
		return nil, err
	}

	if err = h.taskSender.SendMessage(ctx, h.taskQueueUrl, task.ID); err != nil {
		updErr := h.taskRepository.UpdateTask(
			ctx,
			&repository.UpdateTaskInput{ID: task.ID, Status: models.TaskStatusError.Pointer()},
		)
		if updErr != nil {
			return nil, fmt.Errorf(
				"failed to update task status while processing sendMessage err: %w: %s",
				err, updErr,
			)
		}
		return nil, err
	}

	return &oas.CreateTaskOutput{ID: task.ID}, nil
}

// GetTaskStatus returns task status.
func (h *handler) GetTaskStatus(ctx context.Context, params oas.GetTaskStatusParams) (oas.GetTaskStatusRes, error) {
	task, exists, err := h.taskRepository.GetTask(ctx, params.TaskID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return &oas.GetTaskStatusNotFound{}, nil
	}

	var headers oas.OptTaskStatusOutputHeaders
	if task.ResponseHeaders != nil {
		headers = oas.NewOptTaskStatusOutputHeaders(task.ResponseHeaders)
	}
	var statusCode oas.OptInt
	if task.ResponseStatusCode != nil {
		statusCode = oas.NewOptInt(*task.ResponseStatusCode)
	}
	var contentLength oas.OptInt64
	if task.ResponseContentLength != nil {
		contentLength = oas.NewOptInt64(*task.ResponseContentLength)
	}

	return &oas.TaskStatusOutput{
		ID:             task.ID,
		Status:         oas.TaskStatus(task.Status),
		Headers:        headers,
		HTTPStatusCode: statusCode,
		Length:         contentLength,
	}, nil
}
