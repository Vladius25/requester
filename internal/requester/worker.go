package requester

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/go-faster/errors"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"runtime/debug"
	"sync"
	"time"
)

// messageReceiver is an interface for receiving messages from the queue.
type messageReceiver interface {
	VisibilityTimeout() time.Duration
	DecodeMessage(ctx context.Context, queueURL *string, message *sqs.Message, output interface{}) error
	GetMessages(ctx context.Context, input *sqs.ReceiveMessageInput) ([]*sqs.Message, error)
	DeleteMessage(ctx context.Context, queue *string, message *sqs.Message) error
}

// Worker is an implementation of Worker.
type Worker struct {
	queueURL  *string
	workers   int
	receiver  messageReceiver
	processor Processor
	logger    *zap.Logger
}

// NewWorker creates a new worker.
func NewWorker(
	queueURL *string,
	workers int,
	receiver messageReceiver,
	processor Processor,
	logger *zap.Logger,
) (*Worker, error) {
	if workers > 10 {
		return nil, errors.New("max workers count is 10")
	}
	if queueURL == nil {
		return nil, errors.New("must specify queueURL")
	}
	if receiver == nil {
		return nil, errors.New("must specify Receiver")
	}
	if processor == nil {
		return nil, errors.New("must specify Processor")
	}
	if logger == nil {
		return nil, errors.New("must specify logger")
	}
	return &Worker{
		queueURL:  queueURL,
		workers:   workers,
		receiver:  receiver,
		processor: processor,
		logger:    logger,
	}, nil
}

// WatchMessages starts a polling loop for messages from the queue,
// followed by their processing.
func (w *Worker) WatchMessages(ctx context.Context) {
	var wg sync.WaitGroup
	defer wg.Wait()
	defer w.handlePanic()

	messages := make(chan *sqs.Message)

	for i := 0; i < w.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer w.handlePanic()
			w.listenMessages(ctx, messages)
		}()
	}

	for {
		output, err := w.receiveMessages(ctx)
		if ctx.Err() != nil {
			w.logger.Info("Termination of the worker due to context cancellation")
			return
		}
		if err != nil {
			w.logger.Error("Error reading messages from the queue", zap.Error(err))
			continue
		}
		for _, message := range output {
			select {
			case <-ctx.Done():
				return
			case messages <- message:
			}
		}
	}
}

// listenMessages listens for messages from the queue.
func (w *Worker) listenMessages(ctx context.Context, messages chan *sqs.Message) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-messages:
			w.handleMessage(ctx, msg)
		}
	}
}

// receiveMessages receives messages.
func (w *Worker) receiveMessages(ctx context.Context) ([]*sqs.Message, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	input := &sqs.ReceiveMessageInput{
		QueueUrl:            w.queueURL,
		MaxNumberOfMessages: aws.Int64(int64(w.workers)),
		WaitTimeSeconds:     aws.Int64(10),
		VisibilityTimeout:   aws.Int64(int64(w.receiver.VisibilityTimeout() / time.Second)),
		AttributeNames:      []*string{aws.String(sqs.MessageSystemAttributeNameApproximateReceiveCount)},
	}

	return w.receiver.GetMessages(ctx, input)
}

// handleMessage performs processing of a message from the queue.
// The function is intended to be launched in a goroutine, within a pool of similar goroutine-workers.
func (w *Worker) handleMessage(ctx context.Context, sqsMsg *sqs.Message) {
	start := time.Now()
	logg := w.logger.With(zap.String("MessageId", *sqsMsg.MessageId))
	logg.Info("Message received for processing")

	var taskID uuid.UUID
	if err := w.receiver.DecodeMessage(ctx, w.queueURL, sqsMsg, &taskID); err != nil {
		logg.Error("Error decoding the message", zap.Error(err))
		return
	}

	if err := w.processor.WithLogger(logg).ProcessTask(ctx, taskID); err != nil {
		logg.Error("Error processing the message", zap.Error(err))
		return
	}

	if err := w.receiver.DeleteMessage(ctx, w.queueURL, sqsMsg); err != nil {
		logg.Error("Error deleting the message", zap.Error(err))
		return
	}

	logg.With(zap.Duration("Duration", time.Since(start))).
		Info("Successfully processed the message")
}

// handlePanic catches panic and logs the error.
func (w *Worker) handlePanic() {
	if r := recover(); r != nil {
		var err error
		var ok bool
		if err, ok = r.(error); !ok {
			err = fmt.Errorf("%v", r)
		}
		w.logger.Error(
			"Panic",
			zap.Error(err),
			zap.ByteString("stacktrace", debug.Stack()),
		)
	}
}
