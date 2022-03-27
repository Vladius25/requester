package main

import (
	"context"
	_ "github.com/joho/godotenv/autoload"
	"go.uber.org/zap"
	"net"
	"net/http"
	"os"
	"os/signal"
	"requester/internal/logger"
	"requester/internal/queue"
	"requester/internal/repository"
	"requester/internal/requester"
	"syscall"
	"time"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logg := logger.New()
	defer logg.Sync()
	logg.Info("Starting worker...")

	dbConfig := repository.MustConfig(repository.LoadConfig())
	dbPool := repository.MustPool(repository.SetupPool(ctx, dbConfig))
	defer dbPool.Close()

	queueConfig := queue.MustConfig(queue.LoadConfig(
		queue.SQSParams{
			VisibilityTimeout:  5 * time.Minute,
			MaxMessageAttempts: 3,
		},
	))
	queueSvc := queue.New(&queueConfig)

	cfg := requester.MustConfig(requester.LoadConfig())
	taskQueueUrl, err := queueSvc.GetQueueURL(ctx, cfg.TaskQueue)
	if err != nil {
		logg.Fatal("Unable to get task queue url", zap.Error(err))
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: 5 * time.Second,
		},
	}
	processor, err := requester.New(repository.NewTaskDB(dbPool), client, logg)
	if err != nil {
		logg.Fatal("Unable to create processor", zap.Error(err))
	}
	instance, err := requester.NewWorker(taskQueueUrl, cfg.Workers, queueSvc, processor, logg)
	if err != nil {
		logg.Fatal("Unable to create worker", zap.Error(err))
	}

	logg.Info("Waiting for messages")
	go instance.WatchMessages(ctx)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	<-signalChan

	logg.Info("Stopping worker...")
}
