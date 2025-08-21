// Package logger provides logging utilities for the Bonsai application.
package logger

import (
	"context"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

// InitLogging configures the logger. It sets the log level from the LOG_LEVEL environment variable if present.
func InitLogging() {
	logrus.Info("....Configuring Logger....")
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "debug" // default if not set
	}
	setLogLevel(logLevel)
	// Optionally, you can add log format from env as well
	logFormat := os.Getenv("LOG_FORMAT")
	if strings.ToLower(logFormat) == "json" {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	}
}

func setLogLevel(level string) {
	switch strings.ToLower(level) {
	case "trace":
		logrus.SetLevel(logrus.TraceLevel)
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	case "fatal":
		logrus.SetLevel(logrus.FatalLevel)
	case "panic":
		logrus.SetLevel(logrus.PanicLevel)
	default:
		logrus.Infof("NO/Invalid LOGGING_LEVEL is provided, defaulting logging level to DEBUG, provided loggingLevel=[%s]", level)
		logrus.SetLevel(logrus.DebugLevel)
		return
	}
	logrus.Infof("Setting logging level to %s", level)
}

// Info logs an informational message with optional formatting arguments.
func Info(_ context.Context, msg string, args ...interface{}) {
	logrus.Infof(msg, args...)
}

// Debug logs a debug message with optional formatting arguments.
func Debug(_ context.Context, msg string, args ...any) {
	logrus.Debugf(msg, args...)
}

// Error logs an error message with optional formatting arguments.
func Error(_ context.Context, msg string, args ...any) {
	logrus.Errorf(msg, args...)
}

// Trace logs a trace message with optional formatting arguments.
func Trace(_ context.Context, msg string, args ...any) {
	logrus.Tracef(msg, args...)
}

// Warn logs a warning message with optional formatting arguments.
func Warn(_ context.Context, msg string, args ...any) {
	logrus.Warnf(msg, args...)
}

// Fatal logs a fatal message with optional formatting arguments and then exits the application.
func Fatal(_ context.Context, msg string, args ...any) {
	logrus.Fatalf(msg, args...)
}
