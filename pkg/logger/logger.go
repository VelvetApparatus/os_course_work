package logger

import (
	"fmt"
	"log/slog"
	"os"
)

type LoggerOption func(l *slog.Logger)

func WithField(key string, value any) LoggerOption {
	return func(l *slog.Logger) {
		l.With(slog.Any(key, value))
	}
}

func NewLogger(filePath string, opts ...LoggerOption) *slog.Logger {

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Printf("failed to open file: %s\n", err.Error())
		f = os.Stdout
	}

	log := slog.New(slog.NewTextHandler(f, nil))

	for _, opt := range opts {
		opt(log)
	}
	return log
}
