package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

// InitLogger initializes the global logger instance
func InitLogger(development bool) error {
	var config zap.Config
	
	if development {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		config = zap.NewProductionConfig()
	}
	
	// Set log level
	config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	
	// Customize output paths
	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}
	
	logger, err := config.Build()
	if err != nil {
		return err
	}
	
	Logger = logger
	zap.ReplaceGlobals(logger)
	
	return nil
}

// GetLogger returns the global logger instance
func GetLogger() *zap.Logger {
	if Logger == nil {
		// Fallback to a basic logger if not initialized
		Logger = zap.NewNop()
	}
	return Logger
}

// Sync flushes any buffered log entries
func Sync() {
	if Logger != nil {
		_ = Logger.Sync()
	}
}