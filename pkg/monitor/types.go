package monitor

import (
	"context"
)

type Interface interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	SerializeResults(ctx context.Context, junitSuiteName, timeSuffix string) error
}
