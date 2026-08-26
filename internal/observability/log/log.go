package log

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/FreekingDean/gojellyfin/internal/context"
)

type Logger interface {
	Trace(ctx context.Context, msg string, args ...any)
	Debug(ctx context.Context, msg string, args ...any)
	Info(ctx context.Context, msg string, args ...any)
	Warn(ctx context.Context, msg string, args ...any)
	Error(ctx context.Context, msg string, args ...any)
	Fatal(ctx context.Context, msg string, args ...any)
	Internal() Logger
}

type loggerImpl struct {
	writer   *zap.Logger
	internal bool
}

func New() (Logger, error) {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	logger, err := config.Build()

	return &loggerImpl{
		writer:   logger,
		internal: false,
	}, err
}

func (l *loggerImpl) Internal() Logger {
	return &loggerImpl{
		internal: true,
	}
}

type logLevel int

const (
	logLevelTrace logLevel = iota
	logLevelDebug
	logLevelInfo
	logLevelWarn
	logLevelError
	logLevelFatal
)

func (l *loggerImpl) write(ctx context.Context, level logLevel, msg string, args ...any) {
	fields := []zap.Field{}
	if !l.internal {
		fields = append(fields, zap.String("userID", ctx.UserID()))
	}
	l.writer.Log(zapcore.Level(level), fmt.Sprintf(msg, args...), fields...)
}

func (l *loggerImpl) Trace(ctx context.Context, msg string, args ...any) {
	l.write(ctx, logLevelTrace, msg, args...)
}

func (l *loggerImpl) Debug(ctx context.Context, msg string, args ...any) {
	l.write(ctx, logLevelDebug, msg, args...)
}

func (l *loggerImpl) Info(ctx context.Context, msg string, args ...any) {
	l.write(ctx, logLevelInfo, msg, args...)
}

func (l *loggerImpl) Warn(ctx context.Context, msg string, args ...any) {
	l.write(ctx, logLevelWarn, msg, args...)
}

func (l *loggerImpl) Error(ctx context.Context, msg string, args ...any) {
	l.write(ctx, logLevelError, msg, args...)
}

func (l *loggerImpl) Fatal(ctx context.Context, msg string, args ...any) {
	l.write(ctx, logLevelFatal, msg, args...)
	panic(fmt.Sprintf("fatal error: %s", fmt.Sprintf(msg, args...)))
}
