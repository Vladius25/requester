package main

import (
	"context"
	_ "github.com/joho/godotenv/autoload"
	"go.uber.org/zap"
	"net/http"
	"os"
	"os/signal"
	"requester/internal/api"
	"requester/internal/logger"
	"requester/internal/queue"
	"requester/internal/repository"
	"syscall"
	"time"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logg := logger.New()
	defer logg.Sync()
	logg.Info("Starting API server...")

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

	cfg := api.MustConfig(api.LoadConfig())
	taskQueueUrl, err := queueSvc.GetQueueURL(ctx, cfg.TaskQueue)
	if err != nil {
		logg.Fatal("Unable to get task queue url", zap.Error(err))
	}

	h, err := api.NewHandler(&cfg, queueSvc, taskQueueUrl, dbPool, logg)
	if err != nil {
		logg.Fatal("Unable to create API handler", zap.Error(err))
	}
	httpServer := http.Server{
		Addr:         cfg.ListenAddress,
		Handler:      h,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 6 * time.Second,
		IdleTimeout:  3 * time.Second,
	}

	go func() {
		logg.Info("API server starting...", zap.String("address", cfg.ListenAddress))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logg.Fatal("Unable to start API server", zap.Error(err))
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	<-signalChan

	logg.Info("Stopping API server...")
	if err := httpServer.Shutdown(ctx); err != nil {
		logg.Fatal("Unable to shut down API server", zap.Error(err))
	}
}
