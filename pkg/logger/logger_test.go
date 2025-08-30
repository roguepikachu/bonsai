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
