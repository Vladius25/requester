package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"strconv"
	"time"
)

// Service represents SQS service.
type Service struct {
	client *sqs.SQS
	cfg    *Config
}

// New creates new SQS service.
func New(cfg *Config) *Service {
	sqsSession := session.Must(
		session.NewSessionWithOptions(session.Options{
			Config: aws.Config{
				Credentials: credentials.NewEnvCredentials(),
				Region:      aws.String(cfg.Region),
				Endpoint:    aws.String(cfg.URL),
			},
		}),
	)
	return &Service{
		client: sqs.New(sqsSession),
		cfg:    cfg,
	}
}

// Config returns service config.
func (svc *Service) Config() *Config {
	return svc.cfg
}

func (svc *Service) VisibilityTimeout() time.Duration {
	return svc.cfg.VisibilityTimeout
}

// DecodeMessage decodes message.
// If message can't be decoded, it will be deleted.
func (svc *Service) DecodeMessage(ctx context.Context, queueURL *string, message *sqs.Message, output interface{}) error {
	receiveCount, ok := message.Attributes[sqs.MessageSystemAttributeNameApproximateReceiveCount]
	if ok && !svc.cfg.Debug {
		cnt, _ := strconv.Atoi(*receiveCount)
		if cnt > svc.cfg.MaxMessageAttempts {
			err := svc.DeleteMessage(ctx, queueURL, message)
			if err != nil {
				return err
			}
			return errors.New(fmt.Sprintf("message has been received %d times. Deleted", cnt))
		}
	}

	if err := json.Unmarshal([]byte(*message.Body), output); err != nil {
		err = svc.DeleteMessage(ctx, queueURL, message)
		if err != nil {
			return err
		}
		return fmt.Errorf("unable to decode the message: %w", err)
	}

	return nil
}

// GetMessages returns messages from queue.
func (svc *Service) GetMessages(ctx context.Context, input *sqs.ReceiveMessageInput) ([]*sqs.Message, error) {
	msgResult, err := svc.client.ReceiveMessageWithContext(ctx, input)
	if err != nil {
		return nil, err
	}

	return msgResult.Messages, nil
}

// GetQueueURL returns queue URL.
// Creates queue if it doesn't exist.
func (svc *Service) GetQueueURL(ctx context.Context, queue string) (*string, error) {
	urlResult, err := svc.client.GetQueueUrlWithContext(ctx,
		&sqs.GetQueueUrlInput{
			QueueName: &queue,
		})
	if err != nil {
		if errReq, ok := err.(awserr.RequestFailure); ok {
			if errReq.Code() == sqs.ErrCodeQueueDoesNotExist {
				return svc.CreateQueue(ctx, queue)
			}
		}
		return nil, err
	}
	return urlResult.QueueUrl, nil
}

// CreateQueue creates queue.
func (svc *Service) CreateQueue(ctx context.Context, queue string) (*string, error) {
	outputCreateQueue, err := svc.client.CreateQueueWithContext(
		ctx, &sqs.CreateQueueInput{
			QueueName: aws.String(queue),
		},
	)

	return outputCreateQueue.QueueUrl, err
}

// SendMessage sends message to queue.
func (svc *Service) SendMessage(ctx context.Context, queue *string, message interface{}) error {
	messageBody, err := json.Marshal(message)
	if err != nil {
		return err
	}

	_, err = svc.client.SendMessageWithContext(ctx, &sqs.SendMessageInput{
		MessageAttributes: make(map[string]*sqs.MessageAttributeValue),
		MessageBody:       aws.String(string(messageBody)),
		QueueUrl:          queue,
	})
	return err
}

// DeleteMessage deletes message from queue.
func (svc *Service) DeleteMessage(ctx context.Context, queue *string, message *sqs.Message) error {
	_, err := svc.client.DeleteMessageWithContext(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      queue,
		ReceiptHandle: message.ReceiptHandle,
	})
	return err
}
