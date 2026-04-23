package tools

import (
	"context"
	"encoding/json"
)

// PermissionContextProvider lets a tool enrich its permission request with
// extra data the UI can render (for example, a list of file collisions).
// The returned value is marshalled as JSON into the permission payload.
type PermissionContextProvider interface {
	PermissionContext(ctx context.Context, input json.RawMessage) (any, error)
}

// permissionResolutionKey is the private context key used to carry
// resolution data forwarded by the permission checker into a tool's Execute.
type permissionResolutionKey struct{}

// WithPermissionResolution returns a context that carries resolution data
// delivered alongside the user's permission decision. Tools that need the
// resolution should fetch it via PermissionResolutionFromContext.
func WithPermissionResolution(ctx context.Context, data json.RawMessage) context.Context {
	if len(data) == 0 {
		return ctx
	}
	return context.WithValue(ctx, permissionResolutionKey{}, data)
}

// PermissionResolutionFromContext returns the resolution data associated
// with the current tool invocation, or nil if none was provided.
func PermissionResolutionFromContext(ctx context.Context) json.RawMessage {
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Value(permissionResolutionKey{}).(json.RawMessage)
	return v
}

// toolIDKey is the private context key used to correlate a permission
// check with the originating tool_use block. Pre-warmed permission checks
// fire while the model is still streaming the call's input, so the tool
// ID is the only stable handle the UI can use to match progress updates
// to the pending permission card.
type toolIDKey struct{}

// WithToolID returns a context tagged with the originating tool_use ID.
func WithToolID(ctx context.Context, toolID string) context.Context {
	if toolID == "" {
		return ctx
	}
	return context.WithValue(ctx, toolIDKey{}, toolID)
}

// ToolIDFromContext returns the tool_use ID the context was tagged with,
// or "" if none was attached.
func ToolIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(toolIDKey{}).(string)
	return v
}
