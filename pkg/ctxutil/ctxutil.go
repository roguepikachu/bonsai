// Package ctxutil provides helpers for storing and retrieving values in context.
package ctxutil

import "context"

// key is an unexported type to avoid collisions.
type key int

// requestIDKey is the context key for request IDs.
const (
	requestIDKey key = iota
	clientIDKey
)

// WithRequestID returns a new context with the given request ID.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestID extracts the request ID from the context, if set.
func RequestID(ctx context.Context) string {
	if v := ctx.Value(requestIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// WithClientID returns a new context with the given client ID.
func WithClientID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, clientIDKey, id)
}

// ClientID extracts the client ID from the context, if set.
func ClientID(ctx context.Context) string {
	if v := ctx.Value(clientIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
