package requester

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-faster/jx"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"io"
	"net/http"
	"requester/internal/models"
	"requester/internal/repository"
	"strings"
	"testing"
)

func TestProcessorTestSuite(t *testing.T) {
	suite.Run(t, &ProcessorTestSuite{})
}

type ProcessorTestSuite struct {
	suite.Suite
	dbPool    *pgxpool.Pool
	processor processor
}

func (suite *ProcessorTestSuite) SetupSuite() {
	ctx := context.Background()
	httpmock.Activate()

	dbConfig := repository.MustConfig(repository.LoadConfig())
	suite.dbPool = repository.MustPool(repository.SetupPool(ctx, dbConfig))
	logger := zaptest.NewLogger(suite.T(), zaptest.Level(zap.PanicLevel))

	proc, err := New(repository.NewTaskDB(suite.dbPool), http.DefaultClient, logger)
	suite.Require().NoError(err)
	suite.processor = proc.(processor)
}

func (suite *ProcessorTestSuite) TearDownSuite() {
	httpmock.DeactivateAndReset()
	suite.dbPool.Close()
}

func (suite *ProcessorTestSuite) SetupTest() {
	ctx := context.Background()
	tx, err := suite.dbPool.Begin(ctx)
	suite.Require().NoError(err)
	suite.processor.taskRepository = repository.NewTaskDB(tx)
	suite.T().Cleanup(func() {
		suite.Require().NoError(tx.Rollback(ctx))
	})
}

func (suite *ProcessorTestSuite) prepareTask(ctx context.Context) *models.Task {
	suite.T().Helper()
	task, err := suite.processor.taskRepository.CreateTask(
		ctx, &repository.CreateTaskInput{
			Method:  http.MethodPost,
			URL:     "https://example.com",
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    map[string]jx.Raw{"foo": jx.Raw(`"bar"`)},
		},
	)
	suite.Require().NoError(err)
	suite.Require().Equal(models.TaskStatusNew, task.Status)
	return task
}

func (suite *ProcessorTestSuite) prepareHttpMock(
	task *models.Task, err error,
) (wantStatusCode int, wantContentLength int64, wantHeaders map[string][]string) {
	suite.T().Helper()

	wantHeaders = map[string][]string{
		"Header1": {"value1"},
		"Header2": {"value2"},
	}
	wantContentLength = 123
	wantStatusCode = http.StatusAccepted

	httpmock.RegisterResponder(
		task.Method, task.URL,
		func(req *http.Request) (*http.Response, error) {
			for k, v := range task.Headers {
				suite.Equal(v, req.Header.Get(k))
			}
			taskBody, _ := json.Marshal(task.Body)
			reqBody, _ := io.ReadAll(req.Body)
			suite.JSONEq(string(taskBody), string(reqBody))
			response := httpmock.NewStringResponse(wantStatusCode, "body")
			for k, v := range wantHeaders {
				response.Header.Set(k, strings.Join(v, ","))
			}
			response.ContentLength = wantContentLength
			return response, err
		},
	)

	suite.T().Cleanup(func() {
		httpmock.Reset()
	})

	return wantStatusCode, wantContentLength, wantHeaders
}

func (suite *ProcessorTestSuite) Test_updateTask() {
	ctx := context.Background()
	taskWithResponse := &models.TaskWithResponseData{Task: *suite.prepareTask(ctx)}

	tests := []struct {
		name       string
		newStatus  models.TaskStatus
		wantStatus models.TaskStatus
	}{
		{"in_process", models.TaskStatusInProcess, models.TaskStatusInProcess},
		{"done", models.TaskStatusDone, models.TaskStatusDone},
		{"error_after_done", models.TaskStatusError, models.TaskStatusDone},
	}
	var err error
	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.Require().NoError(
				suite.processor.updateTask(
					ctx,
					taskWithResponse,
					&repository.UpdateTaskInput{Status: tt.newStatus.Pointer()},
				),
			)
			suite.Require().Equal(tt.wantStatus, taskWithResponse.Status)
			var exists bool
			taskWithResponse, exists, err = suite.processor.taskRepository.GetTask(ctx, taskWithResponse.ID)
			suite.Require().NoError(err)
			suite.Require().True(exists)
			suite.Require().Equal(tt.wantStatus, taskWithResponse.Status)
		})
	}
}

func (suite *ProcessorTestSuite) Test_processTask_makeRequest() {
	ctx := context.Background()
	task := suite.prepareTask(ctx)
	wantStatusCode, wantContentLength, wantHeaders := suite.prepareHttpMock(task, nil)

	response, err := suite.processor.makeRequest(ctx, task)
	suite.Require().NoError(err)

	suite.Equal(wantStatusCode, response.StatusCode)
	suite.Equal(wantContentLength, response.ContentLength)
	suite.EqualValues(wantHeaders, response.Header)
}

func (suite *ProcessorTestSuite) Test_processTask_ProcessTask_error() {
	ctx := context.Background()
	task := suite.prepareTask(ctx)
	suite.prepareHttpMock(task, errors.New("request failed"))

	suite.Require().Error(suite.processor.ProcessTask(ctx, task.ID))

	taskWithResponse, exists, err := suite.processor.taskRepository.GetTask(ctx, task.ID)
	suite.Require().NoError(err)
	suite.Require().True(exists)
	suite.Require().Equal(models.TaskStatusError, taskWithResponse.Status)
}

func (suite *ProcessorTestSuite) Test_processTask_ProcessTask_ok() {
	ctx := context.Background()
	task := suite.prepareTask(ctx)
	wantStatusCode, wantContentLength, wantHeaders := suite.prepareHttpMock(task, nil)

	suite.Require().NoError(suite.processor.ProcessTask(ctx, task.ID))

	taskWithResponse, exists, err := suite.processor.taskRepository.GetTask(ctx, task.ID)
	suite.Require().NoError(err)
	suite.Require().True(exists)

	suite.Equal(models.TaskStatusDone, taskWithResponse.Status)
	suite.Equal(wantStatusCode, *taskWithResponse.ResponseStatusCode)
	suite.Equal(wantContentLength, *taskWithResponse.ResponseContentLength)
	suite.Equal(wantHeaders, taskWithResponse.ResponseHeaders)
}
