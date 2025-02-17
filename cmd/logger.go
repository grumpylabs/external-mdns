package cmd

import (
	"os"

	"github.com/grumpylabs/external-mdns/cmd/config"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger() (*zap.Logger, error) {
	var logLevel zapcore.Level = zapcore.InfoLevel

	if viper.GetBool(config.Debug) {
		logLevel = zapcore.DebugLevel
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	encoder := zapcore.NewJSONEncoder(encoderConfig)

	logOutput := zapcore.Lock(os.Stdout)
	core := zapcore.NewCore(encoder, logOutput, logLevel)

	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return logger, nil
}
