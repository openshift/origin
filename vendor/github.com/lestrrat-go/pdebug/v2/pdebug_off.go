// +build !debug,!debug0

package pdebug

import (
	"context"
	"io"
)

const Enabled = false

func Context(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func WithClock(ctx context.Context, _ Clock) context.Context {
	return ctx
}

func WithID(ctx context.Context, _ string) context.Context {
	return ctx
}

func WithOutput(ctx context.Context, _ io.Writer) context.Context {
	return ctx
}

func WithPrefix(ctx context.Context, _ string) context.Context {
	return ctx
}

func WithTimestamp(ctx context.Context, _ bool) context.Context {
	return ctx
}

type MarkerGuard struct {
}

func Marker(_ context.Context, _ string, _ ...interface{}) *MarkerGuard {
	return nil
}

func (mg *MarkerGuard) BindError(_ *error) *MarkerGuard {
	return nil
}

func (mg *MarkerGuard) End() {}

func Printf(_ context.Context, _ string, _ ...interface{}) {}
