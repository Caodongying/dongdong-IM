package trace

import "context"

// context的key得是私有类型，不然可能与第三方库的key冲突

type traceIDKey string

const TraceIDKey traceIDKey = "trace_id"

// 向context中注入trace_id
func SetTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// 从context中读取trace_id
func GetTraceID(ctx context.Context) string {
	val, ok := ctx.Value(TraceIDKey).(string)

	if !ok {
		return "unknown"
	}

	return val
}