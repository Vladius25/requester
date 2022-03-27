package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/go-faster/jx"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"net/http"
	"net/http/httptest"
	"requester/internal/api/oas"
	"requester/internal/repository"
	"testing"
)

func TestTasksTestSuite(t *testing.T) {
	suite.Run(t, &TasksTestSuite{})
}

type TasksTestSuite struct {
	suite.Suite
	handler *handler
	server  *oas.Server
}

func (suite *TasksTestSuite) serve(req *http.Request) *http.Response {
	suite.T().Helper()
	w := httptest.NewRecorder()
	suite.server.ServeHTTP(w, req)
	return w.Result()

}

func (suite *TasksTestSuite) SetupSuite() {
	config := MustConfig(LoadConfig())
	url := "sqs://test-queue"
	logger := zaptest.NewLogger(suite.T(), zaptest.Level(zap.PanicLevel))

	var err error
	suite.server, suite.handler, err = newServer(&config, &testTaskSender{}, &url, dbPool, logger)
	suite.Require().NoError(err)
}

func (suite *TasksTestSuite) SetupTest() {
	suite.handler.taskSender = &testTaskSender{}

	ctx := context.Background()
	tx, err := dbPool.Begin(ctx)
	suite.Require().NoError(err)
	suite.handler.taskRepository = repository.NewTaskDB(tx)
	suite.T().Cleanup(func() {
		suite.Require().NoError(tx.Rollback(ctx))
	})
}

func (suite *TasksTestSuite) getValidTaskInput() oas.CreateTaskInput {
	return oas.CreateTaskInput{
		Method:  http.MethodGet,
		URL:     "https://example.com",
		Headers: oas.NewOptCreateTaskInputHeaders(map[string]string{"Content-Type": "application/json"}),
		Body:    oas.NewOptCreateTaskInputBody(map[string]jx.Raw{"field": jx.Raw(`"test"`)}),
	}
}

func (suite *TasksTestSuite) Test_HandleCreateTask_ok() {
	ctx := context.Background()
	sender := suite.handler.taskSender.(*testTaskSender)
	sender.On("SendMessage", mock.Anything, suite.handler.taskQueueUrl, mock.Anything).
		Return(nil)
	defer sender.AssertExpectations(suite.T())

	data := suite.getValidTaskInput()
	dataBytes, _ := json.Marshal(data)

	req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewReader(dataBytes))
	req.Header.Set("Content-Type", "application/json")

	resp := suite.serve(req)
	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	response := oas.CreateTaskOutput{}
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&response))

	task, exists, err := suite.handler.taskRepository.GetTask(ctx, response.ID)
	suite.Require().NoError(err)
	suite.Require().True(exists)

	suite.Equal(response.ID, task.ID)
	suite.EqualValues(data.Method, task.Method)
	suite.EqualValues(data.URL, task.URL)
	suite.EqualValues(data.Headers.Value, task.Headers)
	suite.EqualValues(data.Body.Value, task.Body)
}

func (suite *TasksTestSuite) Test_HandleCreateTask_queueError() {
	type errorResponse struct {
		ErrorMessage string `json:"error_message"`
	}

	sender := suite.handler.taskSender.(*testTaskSender)
	sender.On("SendMessage", mock.Anything, suite.handler.taskQueueUrl, mock.Anything).
		Return(errors.New("test error"))
	defer sender.AssertExpectations(suite.T())

	dataBytes, _ := json.Marshal(suite.getValidTaskInput())
	req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewReader(dataBytes))
	req.Header.Set("Content-Type", "application/json")

	resp := suite.serve(req)
	suite.Require().Equal(http.StatusInternalServerError, resp.StatusCode)
	suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&errorResponse{}))
}

func (suite *TasksTestSuite) Test_HandleCreateTask_badRequest() {
	type errorResponse struct {
		ErrorMessage string `json:"error_message"`
	}
	tests := []struct {
		name    string
		reqData []byte
	}{
		{
			"empty_body",
			[]byte(`{}`),
		},
		{
			"bad_values",
			[]byte(`{"url": "/test", "method": "TEST"}`),
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewReader(tt.reqData))
			req.Header.Set("Content-Type", "application/json")

			resp := suite.serve(req)
			suite.Equal(http.StatusBadRequest, resp.StatusCode)
			suite.NoError(json.NewDecoder(resp.Body).Decode(&errorResponse{}))
		})
	}
}

func (suite *TasksTestSuite) Test_HandleGetTask() {
	input := suite.getValidTaskInput()
	task, err := suite.handler.taskRepository.CreateTask(
		context.Background(),
		&repository.CreateTaskInput{
			Method:  string(input.Method),
			URL:     input.URL,
			Headers: input.Headers.Value,
			Body:    input.Body.Value,
		},
	)
	suite.Require().NoError(err)

	tests := []struct {
		name               string
		taskID             string
		responseStatusCode int
	}{
		{
			"invalid_id",
			"test",
			http.StatusBadRequest,
		},
		{
			"not_existing_id",
			"1c14c6bb-8c66-4797-a626-c0be85c8fa8f",
			http.StatusNotFound,
		},
		{
			"valid",
			task.ID.String(),
			http.StatusOK,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			req := httptest.NewRequest(http.MethodGet, "/tasks/"+tt.taskID, nil)

			resp := suite.serve(req)
			suite.Equal(tt.responseStatusCode, resp.StatusCode)

			if tt.responseStatusCode == http.StatusOK {
				data := oas.TaskStatusOutput{}
				suite.Require().NoError(json.NewDecoder(resp.Body).Decode(&data))

				suite.Equal(task.ID, data.ID)
				suite.EqualValues(task.Status, data.Status)
				suite.False(data.HTTPStatusCode.Set)
				suite.False(data.Headers.Set)
				suite.False(data.Length.Set)
			}
		})
	}
}
