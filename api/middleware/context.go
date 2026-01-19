package middleware

import "context"

type contextKey string

const (
	ctxUserID  contextKey = "user_id"
	ctxRole    contextKey = "actor_role"
	ctxStoreID contextKey = "store_id"
)

func UserIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(ctxUserID).(string); ok {
		return v
	}
	return ""
}

func RoleFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(ctxRole).(string); ok {
		return v
	}
	return ""
}

func StoreIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(ctxStoreID).(string); ok {
		return v
	}
	return ""
}
