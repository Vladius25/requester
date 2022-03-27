package api

import (
	"context"
	"github.com/jackc/pgx/v4/pgxpool"
	_ "github.com/joho/godotenv/autoload"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"net/http"
	"net/http/httptest"
	"requester/internal/repository"
	"testing"
)

var dbPool *pgxpool.Pool

type testTaskSender struct {
	mock.Mock
}

func (s *testTaskSender) SendMessage(ctx context.Context, url *string, data interface{}) error {
	args := s.Called(ctx, url, data)
	return args.Error(0)
}

func TestMain(m *testing.M) {
	ctx := context.Background()

	dbConfig := repository.MustConfig(repository.LoadConfig())
	dbPool = repository.MustPool(repository.SetupPool(ctx, dbConfig))
	defer dbPool.Close()

	m.Run()
}

func Test_HandleHealthStatus(t *testing.T) {
	config := MustConfig(LoadConfig())
	url := "sqs://test-queue"
	logger := zaptest.NewLogger(t, zaptest.Level(zap.PanicLevel))

	h, err := NewHandler(&config, &testTaskSender{}, &url, dbPool, logger)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, config.MountPrefix+"/health", nil)
	writer := httptest.NewRecorder()

	h.ServeHTTP(writer, req)
	resp := writer.Result()

	require.Equal(t, http.StatusOK, resp.StatusCode)
}
