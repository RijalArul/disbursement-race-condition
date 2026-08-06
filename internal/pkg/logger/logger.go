package logger

import (
	"context"
	"log/slog"
	"os"
)

type ctxKey struct{}

var base *slog.Logger

func Init(level string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	base = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}

// WithRequestID attaches a request-scoped logger to ctx so downstream
// handlers/services/repositories retrieve it via FromCtx instead of the
// package-level logger, keeping request_id correlated in every log line.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	l := base
	if l == nil {
		Init("info")
		l = base
	}
	l = l.With(slog.String("request_id", requestID))
	return context.WithValue(ctx, ctxKey{}, l)
}

func FromCtx(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return l
	}
	if base == nil {
		Init("info")
	}
	return base
}
