package tracer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

// reqIDKey is a custom type to prevent context key collisions
type reqIDKey string

const requestIDKey reqIDKey = "request_id"

// GenerateRequestID creates a random 8-byte hex string (e.g., "a1b2c3d4e5f60718")
func GenerateRequestID() string {
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// ContextWithRequestID embeds the ID into the HTTP context
func ContextWithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// GetRequestID extracts the ID from the context. Returns empty string if not found.
func GetRequestID(ctx context.Context) string {
	id, ok := ctx.Value(requestIDKey).(string)
	if !ok {
		return ""
	}
	return id
}
