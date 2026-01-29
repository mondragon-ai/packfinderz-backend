package logger

import (
	"context"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/angelmondragon/packfinderz-backend/pkg/env"
	"github.com/rs/zerolog"
)

// Options configures the structured logger.
type Options struct {
	ServiceName string
	Level       zerolog.Level
	WarnStack   bool
	Output      io.Writer
}

type Logger struct {
	base      *zerolog.Logger
	warnStack bool
}

type ctxKey struct{}

func New(opts Options) *Logger {
	if opts.Level == zerolog.NoLevel {
		opts.Level = zerolog.InfoLevel
	}

	var output io.Writer = opts.Output
	if output == nil {
		output = os.Stdout
	}
	var format = env.Get("LOG_FORMAT", "json")
	if format == "console" {
		output = zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: "15:04:05",
			NoColor:    false,
		}
	}

	zerolog.TimeFieldFormat = time.RFC3339Nano

	logger := zerolog.
		New(output).
		With().
		Timestamp().
		Str("service", opts.ServiceName).
		Logger().
		Level(opts.Level)

	return &Logger{
		base:      &logger,
		warnStack: opts.WarnStack,
	}
}

func ParseLevel(value string) zerolog.Level {
	levelString := strings.ToLower(strings.TrimSpace(value))
	if levelString == "" {
		return zerolog.InfoLevel
	}
	if lvl, err := zerolog.ParseLevel(levelString); err == nil {
		return lvl
	}
	return zerolog.InfoLevel
}

func (l *Logger) loggerFromContext(ctx context.Context) *zerolog.Logger {
	if ctx == nil {
		return l.base
	}
	if entry, ok := ctx.Value(ctxKey{}).(*zerolog.Logger); ok {
		return entry
	}
	return l.base
}

func (l *Logger) attach(ctx context.Context, entry zerolog.Logger) context.Context {
	entr := entry
	return context.WithValue(ctx, ctxKey{}, &entr)
}

func (l *Logger) WithField(ctx context.Context, key string, value any) context.Context {
	entry := l.loggerFromContext(ctx)
	return l.attach(ctx, entry.With().Interface(key, value).Logger())
}

func (l *Logger) WithFields(ctx context.Context, fields map[string]any) context.Context {
	entry := l.loggerFromContext(ctx)
	builder := entry.With()
	for k, v := range fields {
		builder = builder.Interface(k, v)
	}
	return l.attach(ctx, builder.Logger())
}

func (l *Logger) WithRequestID(ctx context.Context, requestID string) context.Context {
	return l.WithField(ctx, "request_id", requestID)
}

func (l *Logger) WithUserID(ctx context.Context, userID string) context.Context {
	return l.WithField(ctx, "user_id", userID)
}

func (l *Logger) WithStoreID(ctx context.Context, storeID string) context.Context {
	return l.WithField(ctx, "store_id", storeID)
}

func (l *Logger) WithActorRole(ctx context.Context, role string) context.Context {
	return l.WithField(ctx, "actor_role", role)
}

func (l *Logger) Info(ctx context.Context, msg string) {
	l.loggerFromContext(ctx).Info().Msg(msg)
}

func (l *Logger) Warn(ctx context.Context, msg string) {
	event := l.loggerFromContext(ctx).Warn()
	if l.warnStack {
		event = event.Str("stack", stackTrace())
	}
	event.Msg(msg)
}

func (l *Logger) Error(ctx context.Context, msg string, err error) {
	event := l.loggerFromContext(ctx).Error()
	if err != nil {
		event = event.Err(err)
	}
	event.Str("stack", stackTrace()).Msg(msg)
}

func stackTrace() string {
	return strings.TrimSpace(string(debug.Stack()))
}
