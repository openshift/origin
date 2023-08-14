package ci

import (
	"context"
	"sync"

	"github.com/openshift/origin/pkg/disruption/backend/sampler"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"k8s.io/kubernetes/test/e2e/framework"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/events"
)

type testRemoteFactory struct {
	dependency dependency
	err        error
}

// RemoteSampler has the machinery to start disruption monitor in the cluster
type RemoteSampler struct {
	lock   sync.Mutex
	cancel context.CancelFunc
	config *rest.Config
}

func (bs *RemoteSampler) GetTargetServerName() string {
	return ""
}

func (bs *RemoteSampler) GetLoadBalancerType() string {
	return ""
}

func (bs *RemoteSampler) GetConnectionType() monitorapi.BackendConnectionType {
	return ""
}

func (bs *RemoteSampler) GetProtocol() string {
	return ""
}

func (bs *RemoteSampler) GetDisruptionBackendName() string {
	return ""
}

func (bs *RemoteSampler) GetLocator() monitorapi.Locator {
	return monitorapi.Locator{}
}

func (bs *RemoteSampler) GetURL() (string, error) {
	return "", nil
}

func (bs *RemoteSampler) RunEndpointMonitoring(ctx context.Context, m monitorapi.RecorderWriter, eventRecorder events.EventRecorder) error {
	ctx, cancel := context.WithCancel(ctx)
	bs.lock.Lock()
	bs.cancel = cancel
	bs.lock.Unlock()

	if eventRecorder == nil {
		fakeEventRecorder := events.NewFakeRecorder(100)
		// discard the events
		go func() {
			for {
				select {
				case <-fakeEventRecorder.Events:
				case <-ctx.Done():
					return
				}
			}
		}()
		eventRecorder = fakeEventRecorder
	}

	<-ctx.Done()
	return nil

}

func (bs *RemoteSampler) StartEndpointMonitoring(ctx context.Context, m monitorapi.RecorderWriter, eventRecorder events.EventRecorder) error {
	framework.Logf("DisruptionTest: starting in-cluster monitors")
	return sampler.StartInClusterMonitors(ctx, bs.config)
}

func (bs *RemoteSampler) Stop() {
	bs.lock.Lock()
	cancel := bs.cancel
	bs.lock.Unlock()

	if cancel != nil {
		cancel()
	}
}

// NewInClusterMonitorTestFactory returns a shared disruption test factory that uses
// the given rest Config object to create new disruption test instances.
func NewInClusterMonitorTestFactory(config *rest.Config) Factory {
	return &testRemoteFactory{
		dependency: &restConfigDependency{
			config: config,
		},
	}
}

func (b *testRemoteFactory) New(_ TestConfiguration) (Sampler, error) {
	if b.err != nil {
		return nil, b.err
	}
	return &RemoteSampler{config: b.dependency.GetRestConfig()}, nil
}
