// Package logger provides logging utilities for the Bonsai application.
package logger

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	ctxutil "github.com/roguepikachu/bonsai/internal/utils"
	"github.com/sirupsen/logrus"
)

// InitLogging configures the logger. It sets the log level from the LOG_LEVEL environment variable if present.
func InitLogging() {
	// Always log to stdout for container-friendly behavior
	logrus.SetOutput(os.Stdout)
	logrus.Info("configuring logger")
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "debug" // default if not set
	}
	setLogLevel(logLevel)
	// Optionally, you can add log format from env as well
	logFormat := os.Getenv("LOG_FORMAT")
	if strings.ToLower(logFormat) == "json" {
		logrus.SetFormatter(&logrus.JSONFormatter{TimestampFormat: time.RFC3339Nano})
	} else {
		logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true, TimestampFormat: "15:04:05"})
	}
	// Enable caller reporting when requested
	if v := os.Getenv("LOG_CALLER"); v == "1" || strings.EqualFold(v, "true") {
		logrus.SetReportCaller(true)
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
		logrus.Infof("invalid LOG_LEVEL provided, defaulting to DEBUG, provided level=[%s]", level)
		logrus.SetLevel(logrus.DebugLevel)
		return
	}
	logrus.Infof("Setting logging level to %s", level)
}

// Info logs an informational message with optional formatting arguments. If a request ID is present in the context, it is included in the log.
func Info(ctx context.Context, msg string, args ...any) {
	reqID := ctxutil.RequestID(ctx)
	clientID := ctxutil.ClientID(ctx)
	if reqID != "" || clientID != "" {
		entry := logrus.WithFields(logrus.Fields{})
		if reqID != "" {
			entry = entry.WithField("requestId", reqID)
		}
		if clientID != "" {
			entry = entry.WithField("clientId", clientID)
		}
		if len(args) > 0 {
			entry.Infof(msg, args...)
			return
		}
		entry.Info(msg)
		return
	}
	if len(args) > 0 {
		logrus.Infof(msg, args...)
		return
	}
	logrus.Info(msg)
}

// Debug logs a debug message with optional formatting arguments. If a request ID is present in the context, it is included in the log.
func Debug(ctx context.Context, msg string, args ...any) {
	reqID := ctxutil.RequestID(ctx)
	clientID := ctxutil.ClientID(ctx)
	if reqID != "" || clientID != "" {
		entry := logrus.WithFields(logrus.Fields{})
		if reqID != "" {
			entry = entry.WithField("requestId", reqID)
		}
		if clientID != "" {
			entry = entry.WithField("clientId", clientID)
		}
		if len(args) > 0 {
			entry.Debugf(msg, args...)
			return
		}
		entry.Debug(msg)
		return
	}
	if len(args) > 0 {
		logrus.Debugf(msg, args...)
		return
	}
	logrus.Debug(msg)
}

// Error logs an error message with optional formatting arguments. If a request ID is present in the context, it is included in the log.
func Error(ctx context.Context, msg string, args ...any) {
	reqID := ctxutil.RequestID(ctx)
	clientID := ctxutil.ClientID(ctx)
	if reqID != "" || clientID != "" {
		entry := logrus.WithFields(logrus.Fields{})
		if reqID != "" {
			entry = entry.WithField("requestId", reqID)
		}
		if clientID != "" {
			entry = entry.WithField("clientId", clientID)
		}
		if len(args) > 0 {
			entry.Errorf(msg, args...)
			return
		}
		entry.Error(msg)
		return
	}
	if len(args) > 0 {
		logrus.Errorf(msg, args...)
		return
	}
	logrus.Error(msg)
}

// Trace logs a trace message with optional formatting arguments. If a request ID is present in the context, it is included in the log.
func Trace(ctx context.Context, msg string, args ...any) {
	reqID := ctxutil.RequestID(ctx)
	clientID := ctxutil.ClientID(ctx)
	if reqID != "" || clientID != "" {
		entry := logrus.WithFields(logrus.Fields{})
		if reqID != "" {
			entry = entry.WithField("requestId", reqID)
		}
		if clientID != "" {
			entry = entry.WithField("clientId", clientID)
		}
		if len(args) > 0 {
			entry.Tracef(msg, args...)
			return
		}
		entry.Trace(msg)
		return
	}
	if len(args) > 0 {
		logrus.Tracef(msg, args...)
		return
	}
	logrus.Trace(msg)
}

// Warn logs a warning message with optional formatting arguments. If a request ID is present in the context, it is included in the log.
func Warn(ctx context.Context, msg string, args ...any) {
	reqID := ctxutil.RequestID(ctx)
	clientID := ctxutil.ClientID(ctx)
	if reqID != "" || clientID != "" {
		entry := logrus.WithFields(logrus.Fields{})
		if reqID != "" {
			entry = entry.WithField("requestId", reqID)
		}
		if clientID != "" {
			entry = entry.WithField("clientId", clientID)
		}
		if len(args) > 0 {
			entry.Warnf(msg, args...)
			return
		}
		entry.Warn(msg)
		return
	}
	if len(args) > 0 {
		logrus.Warnf(msg, args...)
		return
	}
	logrus.Warn(msg)
}

// Fatal logs a fatal message with optional formatting arguments and then exits the application. If a request ID is present in the context, it is included in the log.
func Fatal(ctx context.Context, msg string, args ...any) {
	reqID := ctxutil.RequestID(ctx)
	clientID := ctxutil.ClientID(ctx)
	if reqID != "" || clientID != "" {
		entry := logrus.WithFields(logrus.Fields{})
		if reqID != "" {
			entry = entry.WithField("requestId", reqID)
		}
		if clientID != "" {
			entry = entry.WithField("clientId", clientID)
		}
		if len(args) > 0 {
			entry.Fatalf(msg, args...)
			return
		}
		entry.Fatal(msg)
		return
	}
	if len(args) > 0 {
		logrus.Fatalf(msg, args...)
		return
	}
	logrus.Fatal(msg)
}

// With returns a log entry enriched with context-aware fields (requestId, clientId)
// and any additional structured fields provided. This enables idiomatic structured
// logging with consistent correlation across the application.
func With(ctx context.Context, fields map[string]any) *logrus.Entry {
	entry := logrus.WithFields(logrus.Fields{})
	if fields != nil {
		// convert map[string]any to logrus.Fields
		lf := logrus.Fields{}
		for k, v := range fields {
			lf[k] = v
		}
		entry = entry.WithFields(lf)
	}
	if rid := ctxutil.RequestID(ctx); rid != "" {
		entry = entry.WithField("requestId", rid)
	}
	if cid := ctxutil.ClientID(ctx); cid != "" {
		entry = entry.WithField("clientId", cid)
	}
	return entry
}

// WithField is a convenience helper for adding a single structured field.
func WithField(ctx context.Context, key string, value any) *logrus.Entry {
	return With(ctx, map[string]any{key: value})
}

// Sprintf is a helper to safely format optional messages when using Entry.Info etc.
// If format is empty, it returns an empty string.
func Sprintf(format string, args ...any) string {
	if format == "" {
		return ""
	}
	return fmt.Sprintf(format, args...)
}
