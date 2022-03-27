package api

import (
	"context"
	"fmt"
	"github.com/go-faster/errors"
	"github.com/go-faster/jx"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/ogen-go/ogen/ogenerrors"
	"go.uber.org/zap"
	"net/http"
	"requester/internal/api/oas"
	"requester/internal/repository"
	"runtime/debug"
	"time"
)

// taskSender is an interface for sending messages to the task queue.
type taskSender interface {
	SendMessage(ctx context.Context, url *string, data interface{}) error
}

// handler is an implementation of oas.Handler.
type handler struct {
	taskSender     taskSender
	taskQueueUrl   *string
	cfg            *Config
	taskRepository repository.TaskRepository
}

// newServer creates a new server and handler.
func newServer(
	cfg *Config,
	taskSender taskSender,
	taskQueueUrl *string,
	dbPool *pgxpool.Pool,
	logger *zap.Logger,
) (*oas.Server, *handler, error) {
	if cfg == nil {
		return nil, nil, errors.New("must specify *Config")
	}
	if taskSender == nil {
		return nil, nil, errors.New("must specify taskSender")
	}
	if dbPool == nil {
		return nil, nil, errors.New("must specify *pgxpool.Pool")
	}
	h := &handler{
		cfg:            cfg,
		taskSender:     taskSender,
		taskQueueUrl:   taskQueueUrl,
		taskRepository: repository.NewTaskDB(dbPool),
	}
	srv, err := oas.NewServer(h, oas.WithErrorHandler(getErrorHandler(logger)))
	if err != nil {
		return nil, nil, err
	}
	return srv, h, nil
}

// NewHandler creates a new http.Handler.
func NewHandler(
	cfg *Config,
	taskSender taskSender,
	taskQueueUrl *string,
	dbPool *pgxpool.Pool,
	logger *zap.Logger,
) (http.Handler, error) {
	if logger == nil {
		return nil, errors.New("must specify *zap.Logger")
	}

	srv, _, err := newServer(cfg, taskSender, taskQueueUrl, dbPool, logger)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle(cfg.MountPrefix+"/", http.StripPrefix(cfg.MountPrefix, srv))
	docsPath := cfg.MountPrefix + "/docs"
	mux.Handle(docsPath+"/", http.StripPrefix(docsPath, http.FileServer(http.Dir("./api"))))

	return loggingMiddleware{panicMiddleware{mux, logger}, logger}, nil
}

// getErrorHandler returns the api error handler.
func getErrorHandler(logger *zap.Logger) ogenerrors.ErrorHandler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
		code := ogenerrors.ErrorCode(err)
		if code >= http.StatusInternalServerError {
			logger.With(
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Time("time", time.Now()),
			).Error("Internal server error", zap.Error(err))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)

		e := jx.GetEncoder()
		e.ObjStart()
		e.FieldStart("error_message")
		e.Str("An unexpected error occurred while processing the request. " +
			"Please try again later or contact support.")
		e.ObjEnd()

		_, _ = w.Write(e.Bytes())
	}
}

// responseWriter is a wrapper for http.ResponseWriter that allows to get the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// newResponseWriter creates a new responseWriter.
// The default status code is 200.
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK}
}

func (w *responseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// loggingMiddleware is a middleware for logging http requests.
type loggingMiddleware struct {
	Next   http.Handler
	logger *zap.Logger
}

// ServeHTTP provides logging middleware for http requests.
func (m loggingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ww := newResponseWriter(w)
	m.Next.ServeHTTP(ww, r)
	m.logger.With(
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.Int("status_code", ww.statusCode),
		zap.Time("time", time.Now()),
		zap.Duration("duration", time.Since(start)),
	).Info("Request handled")
}

// panicMiddleware is a middleware for recovering from panics.
type panicMiddleware struct {
	Next   http.Handler
	logger *zap.Logger
}

// ServeHTTP provides panic recovery middleware for http requests.
func (m panicMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			w.WriteHeader(http.StatusInternalServerError)
			var err error
			var ok bool
			if err, ok = rec.(error); !ok {
				err = fmt.Errorf("%v", r)
			}
			m.logger.With(
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Time("time", time.Now()),
			).Error("panic recovered", zap.Error(err), zap.ByteString("stack", debug.Stack()))
		}
	}()
	m.Next.ServeHTTP(w, r)
}
