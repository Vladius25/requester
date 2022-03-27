package requester

import (
	"context"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"testing"
	"time"
)

type testMessageReceiver struct {
	mock.Mock
}

func (r *testMessageReceiver) GetMessages(
	ctx context.Context,
	input *sqs.ReceiveMessageInput,
) ([]*sqs.Message, error) {
	args := r.Called(ctx, input)
	return args.Get(0).([]*sqs.Message), args.Error(1)
}

func (r *testMessageReceiver) DeleteMessage(context.Context, *string, *sqs.Message) error {
	return nil
}

func (r *testMessageReceiver) VisibilityTimeout() time.Duration {
	return 20 * time.Minute
}

func (r *testMessageReceiver) DecodeMessage(
	ctx context.Context,
	url *string,
	message *sqs.Message,
	output interface{},
) error {
	args := r.Called(ctx, url, message, output)
	return args.Error(0)
}

type testProcessor struct {
	mock.Mock
}

func (p *testProcessor) ProcessTask(ctx context.Context, taskID uuid.UUID) error {
	args := p.Called(ctx, taskID)
	return args.Error(0)
}

func (p *testProcessor) WithLogger(*zap.Logger) Processor {
	return p
}

func Test_WatchMessages(t *testing.T) {
	url := "sqs://task-queue"
	logger := zaptest.NewLogger(t, zaptest.Level(zap.PanicLevel))
	receiver := &testMessageReceiver{}
	proc := &testProcessor{}
	instance, err := NewWorker(&url, 1, receiver, proc, logger)
	require.NoError(t, err)

	taskID := uuid.New()

	msgId := "test"
	receiver.On("GetMessages", mock.Anything, mock.Anything).
		Return([]*sqs.Message{{MessageId: &msgId}}, nil)

	receiver.On(
		"DecodeMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(nil).Run(func(args mock.Arguments) {
		output := args.Get(3).(*uuid.UUID)
		*output = taskID
	})

	proc.On("ProcessTask", mock.Anything, taskID).Return(nil)

	ctx, cancel := context.WithCancel(context.Background())
	go instance.WatchMessages(ctx)
	time.Sleep(5 * time.Millisecond)
	cancel()

	receiver.AssertExpectations(t)
	proc.AssertExpectations(t)
}
