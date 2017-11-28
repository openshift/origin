package interceptor

import (
	"context"
)

func WithAuthorizationContext(ctx context.Context) context.Context {
	return ctx
}
