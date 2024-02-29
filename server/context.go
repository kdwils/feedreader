package server

import (
	"context"

	"github.com/kdwils/feedreader/storage"
	"go.uber.org/zap"
)

type ctxKey int

const (
	_ ctxKey = iota
	ctxLoggerKey
	ctxOptionsKey
)

func LoggerToContext(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, ctxLoggerKey, logger)
}

func LoggerFromContext(ctx context.Context, with ...zap.Field) *zap.Logger {
	logger, ok := ctx.Value(ctxLoggerKey).(*zap.Logger)
	if !ok {
		return zap.NewNop()
	}

	return logger.With(with...)
}

func OptionsFromContext(ctx context.Context, with ...zap.Field) *storage.Options {
	opts, ok := ctx.Value(ctxOptionsKey).(*storage.Options)
	if !ok {
		return storage.DefaultOptions()
	}

	return opts
}

func OptionsToContext(ctx context.Context, opts *storage.Options) context.Context {
	return context.WithValue(ctx, ctxOptionsKey, opts)
}
