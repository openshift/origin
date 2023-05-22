package ci

import (
	"fmt"
	"net/http"
	"time"

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

type ProtocolType string

const (
	ProtocolHTTP1 ProtocolType = "http1"
	ProtocolHTTP2 ProtocolType = "http2"
)

type LoadBalancerType string

const (
	ExternalLoadBalancerType LoadBalancerType = "external-lb"
	InternalLoadBalancerType LoadBalancerType = "internal-lb"
	ServiceNetworkType       LoadBalancerType = "service-network"
)

type ServerNameType string

const (
	KubeAPIServer      ServerNameType = "kube-apiserver"
	OpenShiftAPIServer ServerNameType = "openshift-apiserver"
)

const (
	KubeAPIServerShutdownIntervalName    = "openshift-kube-apiserver-shutdown"
	KubeAPIServerShutdownIntervalLocator = "disruption/graceful-shutdown server/kube-apiserver"
)

// Factory creates a new instance of a Disruption test from
// the user specified configuration.
type Factory interface {
	New(TestConfiguration) (*BackendSampler, error)
}

// NewDisruptionTestFactory returns a shared disruption test factory that uses
// the given rest Config object to create new disruption test instances.
func NewDisruptionTestFactory(config *rest.Config) Factory {
	return &testFactory{
		dependency: &restConfigDependency{
			config: config,
		},
	}
}

// TestConfiguration allows a user to specify the disruption test parameters
type TestConfiguration struct {
	TestType

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

// TestType defines the disruption test type, the user must
// provide a complete specification for the desired test.
type TestType struct {
	// TargetServer is the server that is is being tested
	TargetServer ServerNameType

	// LoadBalancerType is the type of load balancer through which the
	// disruption test is hitting the target server.
	LoadBalancerType LoadBalancerType

	// ConnectionType specifies whether the underlying TCP connection(s) used
	// by the requests should be new or reused.
	ConnectionType monitorapi.BackendConnectionType

	// Protocol specifies the protocol used by the test,
	// whether it is http/1x or http/2.0
	Protocol ProtocolType
}

func (t TestType) Name() string {
	return fmt.Sprintf("backend-sampler-%s-%s-%s-%s", t.TargetServer, t.Protocol, t.LoadBalancerType, t.ConnectionType)
}

func (t TestType) Locator() string {
	return fmt.Sprintf("disruption/%s type/%s connection/%s protocol/%s server/%s",
		t.Name(), t.LoadBalancerType, t.ConnectionType, t.Protocol, t.TargetServer)
}

func (t TestType) Validate() error {
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

// dependency is an internal interface that facilitates writing a
// unit test for the factory.
type dependency interface {
	NewTransport(TestConfiguration) (http.RoundTripper, error)
	HostName() string
}

type testFactory struct {
	sharedShutdownInterval backendsampler.SampleCollector
	dependency             dependency
}

func (b *testFactory) New(c TestConfiguration) (*BackendSampler, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	rt, err := b.dependency.NewTransport(c)
	if err != nil {
		return nil, err
	}
	client, err := roundtripper.NewClient(roundtripper.Config{
		RT:                           rt,
		ClientTimeout:                c.Timeout,
		UserAgent:                    c.Name(),
		EnableShutdownResponseHeader: c.EnableShutdownResponseHeader,
	})
	if err != nil {
		return nil, err
	}

	requestor := backendsampler.NewHostPathRequestor(b.dependency.HostName(), c.Path)

	setters := make([]backend.WantEventRecorderAndMonitor, 0)
	var want backend.WantEventRecorderAndMonitor
	// we want all test instances using this factory to share
	// a single apiserver shutdown interval tracker.
	if b.sharedShutdownInterval == nil {
		locator := fmt.Sprintf("%s type/%s", KubeAPIServerShutdownIntervalLocator, c.LoadBalancerType)
		// we don't have access to the monitor and event recorder yet
		b.sharedShutdownInterval, want = shutdown.NewSharedShutdownIntervalTracker(nil, nil, nil, locator, KubeAPIServerShutdownIntervalName)
		setters = append(setters, want)
	}

	var collector backendsampler.SampleCollector = b.sharedShutdownInterval
	collector = logger.NewLogger(collector, c.Name(), c.ConnectionType)

	// we don't have access to the monitor and event recorder yet
	collector, want = disruption.NewIntervalTracker(collector, nil, nil, c.Locator(), c.Name(), c.ConnectionType)
	setters = append(setters, want)

	pc := backendsampler.NewSampleProducerConsumer(client, requestor, backendsampler.ResponseCheckerFunc(backendsampler.DefaultResponseChecker), collector)
	runner := sampler.NewWithProducerConsumer(c.SampleInterval, pc)
	backendSampler := &BackendSampler{
		TestConfiguration:           c,
		SampleRunner:                runner,
		wantEventRecorderAndMonitor: setters,
		baseURL:                     requestor.GetBaseURL(),
	}
	return backendSampler, nil
}

// restConfigDependency is used by the factory when we want to create
// a disruption test instance from a rest Config.
type restConfigDependency struct {
	config *rest.Config
}

func (r *restConfigDependency) NewTransport(tc TestConfiguration) (http.RoundTripper, error) {
	var reuseConnection bool
	if tc.ConnectionType == monitorapi.ReusedConnectionType {
		reuseConnection = true
	}
	var useHTTP1 bool
	if tc.Protocol == ProtocolHTTP1 {
		useHTTP1 = true
	}

	rt, err := transport.FromRestConfig(r.config, reuseConnection, tc.Timeout, useHTTP1)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport - %v", err)
	}
	return rt, nil
}
func (r *restConfigDependency) HostName() string { return r.config.Host }
