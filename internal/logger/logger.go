package logger

import (
	"go.uber.org/zap"
	"log"
)

func New() *zap.Logger {
	options := []zap.Option{
		zap.AddStacktrace(zap.FatalLevel),
	}
	logger, err := zap.NewProduction(options...)
	if err != nil {
		log.Fatalf("Error during logger initializtion: %s", err)
	}
	return logger
}
