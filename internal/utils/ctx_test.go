//revive:disable:var-naming - allow test package name 'utils' to match library
package utils

import (
	"context"
	"testing"
)

func TestRequestAndClientID(t *testing.T) {
	ctx := context.Background()
	if got := RequestID(ctx); got != "" {
		t.Fatalf("expected empty request id, got %q", got)
	}
	if got := ClientID(ctx); got != "" {
		t.Fatalf("expected empty client id, got %q", got)
	}
	ctx = WithRequestID(ctx, "rid-1")
	ctx = WithClientID(ctx, "cid-1")
	if got := RequestID(ctx); got != "rid-1" {
		t.Fatalf("request id mismatch, got %q", got)
	}
	if got := ClientID(ctx); got != "cid-1" {
		t.Fatalf("client id mismatch, got %q", got)
	}
}
