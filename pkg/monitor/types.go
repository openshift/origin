package monitor

import (
	"context"

	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/disruption/backend"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

type Interface interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// StartEventIntervalRecorder is non-blocking and must stop on a context cancel.  It is expected to call the recorder
// an is encouraged to use EventIntervals to record edges.  They are often paired with TestSuite.SyntheticEventTests.
type StartEventIntervalRecorderFunc func(ctx context.Context, recorder monitorapi.Recorder, clusterConfig *rest.Config, lb backend.LoadBalancerType) error
