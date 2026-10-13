package monitor

import (
	"context"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

type Interface interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) (ResultState, error)
	SerializeResults(ctx context.Context, junitSuiteName, timeSuffix string) (*junitapi.JUnitTestSuite, error)
}

type ResultState string

var (
	Succeeded ResultState = "Succeeded"
	Failed    ResultState = "Failed"
)
