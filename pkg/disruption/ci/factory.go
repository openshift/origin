package ci

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"

	"github.com/openshift/origin/pkg/disruption/backend"
	"github.com/openshift/origin/pkg/disruption/backend/disruption"
	"github.com/openshift/origin/pkg/disruption/backend/logger"
	"github.com/openshift/origin/pkg/disruption/backend/roundtripper"
	backendsampler "github.com/openshift/origin/pkg/disruption/backend/sampler"
	"github.com/openshift/origin/pkg/disruption/backend/shutdown"
	"github.com/openshift/origin/pkg/disruption/backend/transport"
	"github.com/openshift/origin/pkg/disruption/sampler"
	"github.com/openshift/origin/pkg/monitor/monitorapi"

	"k8s.io/client-go/rest"
)

type ServerNameType string

const (
	KubeAPIServer      ServerNameType = "kube-api"
	OpenShiftAPIServer ServerNameType = "openshift-api"
)

// Factory creates a new instance of a Disruption test from
// the user specified configuration.
type Factory interface {
	New(TestConfiguration) (Sampler, error)
}

// NewDisruptionTestFactory returns a shared disruption test factory that uses
// the given rest Config object to create new disruption test instances.
func NewDisruptionTestFactory(config *rest.Config, kubeClient kubernetes.Interface) Factory {
	return &testFactory{
		dependency: &restConfigDependency{
			config:     config,
			kubeClient: kubeClient,
		},
	}
}

// TestConfiguration allows a user to specify the disruption test parameters
type TestConfiguration struct {
	TestDescriptor

	// Path is the request path that the backend sampler will exercise
	Path string

	// Timeout is the transport timeout, it is the maximum amount of time a
	// dial will wait for a connect to complete.
	// If Deadline is also set, it may fail earlier.
	// NOTE: we infer the client timeout and request deadline from this value,
	//  this is how the current backend sampler works.
	Timeout time.Duration

	// SampleInterval is the interval that the sampler will
	// wait before generating the next sample.
	// NOTE: the current backend sampler implementation defaults
	//  it to 1s.
	SampleInterval time.Duration

	// EnableShutdownResponseHeader indicates whether to include the shutdown
	// response header extractor, this should be true only when the
	// request(s) are being sent to the kube-apiserver.
	EnableShutdownResponseHeader bool
}

// TestDescriptor defines the disruption test type, the user must
// provide a complete specification for the desired test.
type TestDescriptor struct {
	// TargetServer is the server that is is being tested
	TargetServer ServerNameType

	// LoadBalancerType is the type of load balancer through which the
	// disruption test is hitting the target server.
	LoadBalancerType backend.LoadBalancerType

	// ConnectionType specifies whether the underlying TCP connection(s) used
	// by the requests should be new or reused.
	ConnectionType monitorapi.BackendConnectionType

	// Protocol specifies the protocol used by the test,
	// whether it is http/1x or http/2.0
	Protocol backend.ProtocolType
}

func (t TestDescriptor) Name() string {
	return fmt.Sprintf("%s-%s-%s-%s-connections", t.TargetServer, t.Protocol, t.LoadBalancerType, t.ConnectionType)
}

func (t TestDescriptor) DisruptionLocator() monitorapi.Locator {
	return monitorapi.NewLocator().Disruption(
		t.Name(),
		fmt.Sprintf("%v-%v-%v", t.TargetServer, t.Protocol, t.LoadBalancerType),
		string(t.LoadBalancerType),
		string(t.Protocol),
		string(t.TargetServer),
		t.ConnectionType,
	)
}

func (t TestDescriptor) ShutdownLocator() monitorapi.Locator {
	return monitorapi.NewLocator().KubeAPIServerWithLB(string(t.LoadBalancerType))
}

func (t TestDescriptor) Validate() error {
	if len(t.LoadBalancerType) == 0 {
		return fmt.Errorf("LoadBalancerType must have a valid value")
	}
	if len(t.ConnectionType) == 0 {
		return fmt.Errorf("BackendConnectionType must have a valid value")
	}
	if len(t.Protocol) == 0 {
		return fmt.Errorf("Protocol must have a valid value")
	}
	if len(t.TargetServer) == 0 {
		return fmt.Errorf("TargetServer must have a valid value")
	}
	return nil
}
func (t TestDescriptor) GetLoadBalancerType() backend.LoadBalancerType       { return t.LoadBalancerType }
func (t TestDescriptor) GetProtocol() backend.ProtocolType                   { return t.Protocol }
func (t TestDescriptor) GetConnectionType() monitorapi.BackendConnectionType { return t.ConnectionType }
func (t TestDescriptor) GetTargetServerName() string                         { return string(t.TargetServer) }

// dependency is an internal interface that facilitates writing a
// unit test for the factory.
type dependency interface {
	// NewTransport returns a new http.RoundTripper that is appropriate
	// to send requests to the target server.
	NewTransport(TestConfiguration) (http.RoundTripper, error)

	// HostName returns the host name in order to connect to the target server.
	HostName() string

	// GetHostNameDecoder returns the appropriate HostNameDecoder instance.
	GetHostNameDecoder() (backend.HostNameDecoderWithRunner, error)
}

type testFactory struct {
	dependency dependency

	once                   sync.Once
	err                    error
	sharedShutdownInterval backendsampler.SampleCollector
	wantMonitorAndRecorder backend.WantEventRecorderAndMonitorRecorder
	hostNameDecoder        backend.HostNameDecoderWithRunner
}

func (b *testFactory) New(c TestConfiguration) (Sampler, error) {
	if b.err != nil {
		return nil, b.err
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	b.once.Do(func() {
		// we want all test instances using this factory to share
		// a single apiserver shutdown interval tracker.
		b.sharedShutdownInterval, b.wantMonitorAndRecorder = shutdown.NewSharedShutdownIntervalTracker(nil, c, nil, nil)
		b.hostNameDecoder, b.err = b.dependency.GetHostNameDecoder()
	})

	rt, err := b.dependency.NewTransport(c)
	if err != nil {
		return nil, err
	}
	client, err := roundtripper.NewClient(roundtripper.Config{
		RT:                           rt,
		ClientTimeout:                c.Timeout,
		UserAgent:                    c.Name(),
		EnableShutdownResponseHeader: c.EnableShutdownResponseHeader,
		HostNameDecoder:              b.hostNameDecoder,
	})
	if err != nil {
		return nil, err
	}
	requestor := backendsampler.NewHostPathRequestor(b.dependency.HostName(), c.Path)

	// we don't have access to the monitor and event recorder yet
	collector, want := disruption.NewIntervalTracker(b.sharedShutdownInterval, c, nil, nil)
	collector = logger.NewLogger(collector, c)

	pc := backendsampler.NewSampleProducerConsumer(client, requestor, backendsampler.NewResponseChecker(), collector)
	runner := sampler.NewWithProducerConsumer(c.SampleInterval, pc)
	samplerFinished := make(chan struct{})
	backendSampler := &BackendSampler{
		TestConfiguration:           c,
		SampleRunner:                runner,
		wantEventRecorderAndMonitor: []backend.WantEventRecorderAndMonitorRecorder{b.wantMonitorAndRecorder, want},
		baseURL:                     requestor.GetBaseURL(),
		hostNameDecoder:             b.hostNameDecoder,
		samplerFinished:             samplerFinished,
	}
	return backendSampler, nil
}

// restConfigDependency is used by the factory when we want to create
// a disruption test instance from a rest Config.
type restConfigDependency struct {
	config     *rest.Config
	kubeClient kubernetes.Interface
}

func (r *restConfigDependency) NewTransport(tc TestConfiguration) (http.RoundTripper, error) {
	var reuseConnection bool
	if tc.ConnectionType == monitorapi.ReusedConnectionType {
		reuseConnection = true
	}
	var useHTTP1 bool
	if tc.Protocol == backend.ProtocolHTTP1 {
		useHTTP1 = true
	}

	rt, err := transport.FromRestConfig(r.config, reuseConnection, tc.Timeout, useHTTP1)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport - %v", err)
	}
	return rt, nil
}
func (r *restConfigDependency) HostName() string {
	return r.config.Host
}

func (r *restConfigDependency) GetHostNameDecoder() (backend.HostNameDecoderWithRunner, error) {
	return NewAPIServerIdentityToHostNameDecoder(r.kubeClient)
}
