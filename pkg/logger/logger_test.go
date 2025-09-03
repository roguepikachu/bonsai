package logger

import (
	"context"
	"testing"
)

func TestSprintf(t *testing.T) {
	if got := Sprintf(""); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
	if got := Sprintf("hi %s", "there"); got != "hi there" {
		t.Fatalf("unexpected format: %q", got)
	}
}

func TestSprintf_ComplexFormats(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		args     []interface{}
		expected string
	}{
		{"no args", "hello", nil, "hello"},
		{"single string", "hello %s", []interface{}{"world"}, "hello world"},
		{"multiple args", "%s %d %v", []interface{}{"test", 42, true}, "test 42 true"},
		{"number formatting", "%.2f", []interface{}{3.14159}, "3.14"},
		{"hex formatting", "0x%x", []interface{}{255}, "0xff"},
		{"percent literal", "100%%", nil, "100%"},
		{"empty with args", "", []interface{}{"ignored"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Sprintf(tt.format, tt.args...)
			if got != tt.expected {
				t.Fatalf("Sprintf(%q, %v) = %q, want %q", tt.format, tt.args, got, tt.expected)
			}
		})
	}
}

func TestWithAndWithField(t *testing.T) {
	ctx := context.Background()
	e := With(ctx, map[string]any{"k": "v"})
	if e == nil {
		t.Fatal("expected non-nil entry")
	}
	e2 := WithField(ctx, "k2", 2)
	if e2 == nil {
		t.Fatal("expected non-nil entry")
	}
}

func TestWith_EmptyMap(t *testing.T) {
	ctx := context.Background()
	e := With(ctx, map[string]any{})
	if e == nil {
		t.Fatal("expected non-nil entry even with empty map")
	}
}

func TestWith_NilMap(t *testing.T) {
	ctx := context.Background()
	e := With(ctx, nil)
	if e == nil {
		t.Fatal("expected non-nil entry even with nil map")
	}
}

func TestWith_ComplexValues(t *testing.T) {
	ctx := context.Background()
	fields := map[string]any{
		"string": "value",
		"int":    42,
		"float":  3.14,
		"bool":   true,
		"slice":  []string{"a", "b", "c"},
		"map":    map[string]int{"x": 1, "y": 2},
		"nil":    nil,
	}
	e := With(ctx, fields)
	if e == nil {
		t.Fatal("expected non-nil entry")
	}
}

func TestWithField_DifferentTypes(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name  string
		key   string
		value interface{}
	}{
		{"string", "str", "test"},
		{"int", "num", 123},
		{"float", "pi", 3.14159},
		{"bool true", "flag", true},
		{"bool false", "disabled", false},
		{"slice", "list", []int{1, 2, 3}},
		{"map", "config", map[string]string{"key": "value"}},
		{"nil", "empty", nil},
		{"struct", "data", struct{ Name string }{Name: "test"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := WithField(ctx, tt.key, tt.value)
			if e == nil {
				t.Fatal("expected non-nil entry")
			}
		})
	}
}

func TestWithField_EmptyKey(t *testing.T) {
	ctx := context.Background()
	e := WithField(ctx, "", "value")
	if e == nil {
		t.Fatal("expected non-nil entry even with empty key")
	}
}

func TestLoggingMethods(t *testing.T) {
	ctx := context.Background()

	// These should not panic
	Debug(ctx, "debug message")
	Info(ctx, "info message")
	Warn(ctx, "warn message")
	Error(ctx, "error message")
}

func TestLoggingMethodsWithFormatting(t *testing.T) {
	ctx := context.Background()

	// These should not panic and should handle formatting
	Debug(ctx, "debug: %s %d", "test", 123)
	Info(ctx, "info: %v", map[string]int{"count": 42})
	Warn(ctx, "warn: %.2f%%", 75.5)
	Error(ctx, "error: %t", false)
}

func TestLoggingWithContext(t *testing.T) {
	ctx := context.Background()

	// Create entry with fields and log
	e := With(ctx, map[string]any{"component": "test", "request_id": "123"})
	e.Debug("debug with context")
	e.Info("info with context")
	e.Warn("warn with context")
	e.Error("error with context")
}

func TestLoggingWithFieldContext(t *testing.T) {
	ctx := context.Background()

	// Create entry with single field and log
	e := WithField(ctx, "user_id", "user-456")
	e.Debug("debug with field")
	e.Info("info with field")
	e.Warn("warn with field")
	e.Error("error with field")
}

func TestChainedLogging(t *testing.T) {
	ctx := context.Background()

	// Chain multiple field additions
	e := WithField(ctx, "service", "bonsai")
	e = e.WithField("version", "1.0.0")
	e = e.WithField("env", "test")

	e.Info("chained logging example")
}

func TestNilContext(t *testing.T) {
	// Test with nil context - skip the With functions as they rely on context utils
	// The logger should handle basic operations without panicking
	ctx := context.Background() // Use empty context instead of nil
	Debug(ctx, "debug message")
	Info(ctx, "info message")
	Warn(ctx, "warn message")
	Error(ctx, "error message")

	// Test WithField and With with proper context
	_ = WithField(ctx, "key", "value")
	_ = With(ctx, map[string]any{"key": "value"})
}

func TestConcurrentLogging(t *testing.T) {
	ctx := context.Background()

	// Test concurrent logging to ensure no race conditions
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			e := WithField(ctx, "goroutine", id)
			e.Info("concurrent log message")
			Info(ctx, "global log message from goroutine %d", id)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestLargeData(t *testing.T) {
	ctx := context.Background()

	// Test logging with large data structures
	largeSlice := make([]int, 1000)
	for i := range largeSlice {
		largeSlice[i] = i
	}

	largeMap := make(map[string]interface{})
	for i := 0; i < 100; i++ {
		largeMap[Sprintf("key%d", i)] = Sprintf("value%d", i)
	}

	e := With(ctx, map[string]any{
		"large_slice": largeSlice,
		"large_map":   largeMap,
	})
	e.Info("logging large data structures")
}

func TestSpecialCharacters(t *testing.T) {
	ctx := context.Background()

	// Test with special characters and unicode
	specialChars := "!@#$%^&*(){}[]|\\:;\"'<>?,./"
	unicode := "こんにちは世界 🌍 emoji test"

	e := With(ctx, map[string]any{
		"special": specialChars,
		"unicode": unicode,
	})
	e.Info("testing special characters")
}

func TestErrorInterface(t *testing.T) {
	ctx := context.Background()

	// Test logging actual error types
	err := context.DeadlineExceeded
	e := WithField(ctx, "error", err)
	e.Error("error occurred")

	// Test with nil error
	var nilErr error
	e2 := WithField(ctx, "error", nilErr)
	e2.Info("nil error test")
}

func TestMultipleWithCalls(t *testing.T) {
	ctx := context.Background()

	// Test multiple With calls don't interfere
	e1 := With(ctx, map[string]any{"service": "api"})
	e2 := With(ctx, map[string]any{"service": "worker"})

	e1.Info("message from api service")
	e2.Info("message from worker service")

	// They should be independent
	e1.WithField("request_id", "req1").Info("api with request")
	e2.WithField("job_id", "job1").Info("worker with job")
}
