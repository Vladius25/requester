package models

import (
	"github.com/go-faster/jx"
	"github.com/google/uuid"
)

type TaskStatus string

const (
	TaskStatusNew       TaskStatus = "new"
	TaskStatusDone      TaskStatus = "done"
	TaskStatusError     TaskStatus = "error"
	TaskStatusInProcess TaskStatus = "in_process"
)

// Pointer returns *TaskStatus.
func (ts TaskStatus) Pointer() *TaskStatus {
	return &ts
}

// Task to request a 3rd-party service.
type Task struct {
	// ID
	ID uuid.UUID `json:"id"`
	// Processing status
	Status TaskStatus `json:"status"`
	// Request method
	Method string `json:"method"`
	// Request URL
	URL string `json:"url"`
	// RequeTask statusst headers
	Headers map[string]string `json:"headers"`
	// Request body
	Body map[string]jx.Raw `json:"body"`
}

// ResponseData to store response data.
type ResponseData struct {
	// Response status code
	ResponseStatusCode *int `json:"http_status_code"`
	// Response headers
	ResponseHeaders map[string][]string `json:"headers"`
	// Response content length
	ResponseContentLength *int64 `json:"length"`
}

// TaskWithResponseData is a task with response data.
type TaskWithResponseData struct {
	Task
	ResponseData
}
