package monitor

import (
	"context"
)

type Interface interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) (ResultState, error)
	SerializeResults(ctx context.Context, junitSuiteName, timeSuffix string) error
}

type ResultState string

var (
	Succeeded ResultState = "Succeeded"
	Failed    ResultState = "Failed"
)
