package server

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger() (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	cfg.EncoderConfig.StacktraceKey = ""
	return cfg.Build(zap.AddCaller(), zap.AddCallerSkip(1))
}
