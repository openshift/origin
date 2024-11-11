package ci

import (
	"context"
	"fmt"
	"sync"

	"github.com/openshift/origin/pkg/disruption/backend"
	"github.com/openshift/origin/pkg/disruption/sampler"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"k8s.io/kubernetes/test/e2e/framework"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/events"
)

type Sampler interface {
	GetTargetServerName() string
	GetLoadBalancerType() string
	GetConnectionType() monitorapi.BackendConnectionType
	GetProtocol() string
	GetDisruptionBackendName() string
	GetLocator() monitorapi.Locator
	GetURL() (string, error)
	RunEndpointMonitoring(ctx context.Context, m monitorapi.RecorderWriter, eventRecorder events.EventRecorder) error
	StartEndpointMonitoring(ctx context.Context, m monitorapi.RecorderWriter, eventRecorder events.EventRecorder) error
	Stop()
}

// BackendSampler has the machinery to run a disruption test in CI
type BackendSampler struct {
	TestConfiguration
	SampleRunner sampler.Runner

	wantEventRecorderAndMonitor []backend.WantEventRecorderAndMonitorRecorder
	baseURL                     string
	hostNameDecoder             backend.HostNameDecoderWithRunner
	lock                        sync.Mutex
	cancel                      context.CancelFunc
	samplerFinished             chan struct{}
}

func (bs *BackendSampler) GetTargetServerName() string {
	return string(bs.TargetServer)
}

func (bs *BackendSampler) GetLoadBalancerType() string {
	return string(bs.LoadBalancerType)
}

func (bs *BackendSampler) GetConnectionType() monitorapi.BackendConnectionType {
	return bs.ConnectionType
}

func (bs *BackendSampler) GetProtocol() string {
	return string(bs.Protocol)
}

func (bs *BackendSampler) GetDisruptionBackendName() string {
	return bs.Name()
}

func (bs *BackendSampler) GetLocator() monitorapi.Locator {
	return bs.DisruptionLocator()
}

func (bs *BackendSampler) GetURL() (string, error) {
	return bs.baseURL, nil
}

func (bs *BackendSampler) RunEndpointMonitoring(ctx context.Context, m monitorapi.RecorderWriter, eventRecorder events.EventRecorder) error {
	defer close(bs.samplerFinished)

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

	for _, s := range bs.wantEventRecorderAndMonitor {
		s.SetMonitorRecorder(m)
		s.SetEventRecorder(eventRecorder)
	}

	var hostNameDecoderStopped context.Context
	if bs.hostNameDecoder != nil {
		hostNameDecoderStopped = bs.hostNameDecoder.Run(ctx)
	}

	framework.Logf("DisruptionTest: starting sampler name=%s", bs.Name())
	samplerStopped := bs.SampleRunner.Run(ctx)

	<-ctx.Done()

	framework.Logf("DisruptionTest: stop context canceled, waiting for Run to return name=%s", bs.Name())
	<-samplerStopped.Done()
	framework.Logf("DisruptionTest: Run has completed name=%s", bs.Name())
	if hostNameDecoderStopped != nil {
		<-hostNameDecoderStopped.Done()
	}

	return nil
}

func (bs *BackendSampler) StartEndpointMonitoring(ctx context.Context, m monitorapi.RecorderWriter, eventRecorder events.EventRecorder) error {
	if m == nil {
		return fmt.Errorf("monitor is required")
	}

	go func() {
		err := bs.RunEndpointMonitoring(ctx, m, eventRecorder)
		if err != nil {
			utilruntime.HandleError(err)
		}
	}()
	return nil
}

func (bs *BackendSampler) Stop() {
	bs.lock.Lock()
	cancel := bs.cancel
	bs.lock.Unlock()

	if cancel != nil {
		cancel()
	}

	// wait for the sampler to be done
	<-bs.samplerFinished
}
