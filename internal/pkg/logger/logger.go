package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// timestampLayout is RFC3339 in UTC with millisecond precision. The spec's
// example shows second precision; milliseconds are kept because ordering
// log lines within a single request is impossible without them.
const timestampLayout = "2006-01-02T15:04:05.000Z07:00"

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
	base = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:       lvl,
		ReplaceAttr: renameBuiltinKeys,
	}))
}

// renameBuiltinKeys reshapes slog's built-in attributes into the shape the
// spec asks for: slog emits "time" as an RFC3339Nano string and "level" in
// upper case, while the required format is "timestamp" and a lower-case level.
// Only top-level attributes are touched so nested groups keep their own keys.
func renameBuiltinKeys(groups []string, a slog.Attr) slog.Attr {
	if len(groups) > 0 {
		return a
	}
	switch a.Key {
	case slog.TimeKey:
		a.Key = "timestamp"
		a.Value = slog.StringValue(a.Value.Time().UTC().Format(timestampLayout))
	case slog.LevelKey:
		a.Value = slog.StringValue(strings.ToLower(a.Value.String()))
	}
	return a
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
