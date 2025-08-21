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

// Info logs an informational message with optional formatting arguments. If a request ID is present in the context, it is included in the log.
func Info(ctx context.Context, msg string, args ...any) {
	if reqID, ok := ctx.Value("request_id").(string); ok && reqID != "" {
		logrus.WithField("request_id", reqID).Infof(msg, args...)
	} else {
		logrus.Infof(msg, args...)
	}
}

// Debug logs a debug message with optional formatting arguments. If a request ID is present in the context, it is included in the log.
func Debug(ctx context.Context, msg string, args ...any) {
	if reqID, ok := ctx.Value("request_id").(string); ok && reqID != "" {
		logrus.WithField("request_id", reqID).Debugf(msg, args...)
	} else {
		logrus.Debugf(msg, args...)
	}
}

// Error logs an error message with optional formatting arguments. If a request ID is present in the context, it is included in the log.
func Error(ctx context.Context, msg string, args ...any) {
	if reqID, ok := ctx.Value("request_id").(string); ok && reqID != "" {
		logrus.WithField("request_id", reqID).Errorf(msg, args...)
	} else {
		logrus.Errorf(msg, args...)
	}
}

// Trace logs a trace message with optional formatting arguments. If a request ID is present in the context, it is included in the log.
func Trace(ctx context.Context, msg string, args ...any) {
	if reqID, ok := ctx.Value("request_id").(string); ok && reqID != "" {
		logrus.WithField("request_id", reqID).Tracef(msg, args...)
	} else {
		logrus.Tracef(msg, args...)
	}
}

// Warn logs a warning message with optional formatting arguments. If a request ID is present in the context, it is included in the log.
func Warn(ctx context.Context, msg string, args ...any) {
	if reqID, ok := ctx.Value("request_id").(string); ok && reqID != "" {
		logrus.WithField("request_id", reqID).Warnf(msg, args...)
	} else {
		logrus.Warnf(msg, args...)
	}
}

// Fatal logs a fatal message with optional formatting arguments and then exits the application. If a request ID is present in the context, it is included in the log.
func Fatal(ctx context.Context, msg string, args ...any) {
	if reqID, ok := ctx.Value("request_id").(string); ok && reqID != "" {
		logrus.WithField("request_id", reqID).Fatalf(msg, args...)
	} else {
		logrus.Fatalf(msg, args...)
	}
}
