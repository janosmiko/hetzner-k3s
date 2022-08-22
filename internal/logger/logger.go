package logger

import (
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	*zap.Logger
}

func InitLogger() *Logger {
	conf := zap.NewDevelopmentConfig()
	if viper.GetBool("debug") {
		conf.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
		conf.EncoderConfig.FunctionKey = "F"
	} else {
		conf.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
		conf.DisableCaller = true
		conf.EncoderConfig.StacktraceKey = "stacktrace"
		conf.DisableStacktrace = true
	}

	conf.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	conf.EncoderConfig.TimeKey = "timestamp"
	conf.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder

	var (
		logger = Logger{}
		err    error
	)

	logger.Logger, err = conf.Build()
	if err != nil {
		panic(err)
	}

	return &logger
}

type DebugLogger Logger

func (l Logger) DebugWriter() DebugLogger {
	return DebugLogger(l)
}

func (d DebugLogger) Write(p []byte) (n int, err error) {
	d.Debug(string(p))

	return len(p), nil
}
