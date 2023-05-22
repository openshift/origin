package ci

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/disruption/backend"
	"github.com/openshift/origin/pkg/monitor/monitorapi"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestBackendSampler(t *testing.T) {
	duration := time.Second
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		elapsed := duration.Round(time.Second).String()
		w.Header().Set("X-OpenShift-Disruption", fmt.Sprintf("shutdown=true shutdown-delay-duration=1m10s elapsed=%s host=foo", elapsed))
		w.WriteHeader(http.StatusOK)
		duration += time.Second
	}))
	ts.EnableHTTP2 = true
	ts.StartTLS()
	defer ts.Close()

	factory := &testFactory{
		dependency: &testServerDependency{
			server: ts,
		},
	}

	bs, err := factory.New(TestConfiguration{
		TestType: TestType{
			TargetServer:     "test-server",
			LoadBalancerType: ExternalLoadBalancerType,
			ConnectionType:   monitorapi.ReusedConnectionType,
			Protocol:         ProtocolHTTP2,
		},
		Path:                         "/healthz",
		Timeout:                      10 * time.Second,
		SampleInterval:               time.Second,
		EnableShutdownResponseHeader: true,
	})
	if err != nil {
		t.Fatalf("failed to build backend sampler: %v", err)
	}

	t.Logf("setters: %v", bs.wantEventRecorderAndMonitor)

	parent, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitorErrCh := make(chan error, 1)
	go func() {
		err := bs.RunEndpointMonitoring(parent, &fakeMonitor{}, &fakeRecorder{})
		monitorErrCh <- err
	}()
	go func() {
		<-time.After(10 * time.Second)
		bs.Stop()
	}()

	t.Logf("waiting for monitor to be done")
	<-monitorErrCh
}

type testServerDependency struct {
	server *httptest.Server
}

func (d *testServerDependency) NewTransport(tc TestConfiguration) (http.RoundTripper, error) {
	client := d.server.Client()
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("expected an object of %T", &http.Transport{})
	}
	if tc.ConnectionType == monitorapi.NewConnectionType {
		transport.DisableKeepAlives = true
	}
	if tc.Protocol == ProtocolHTTP1 && d.server.EnableHTTP2 {
		return nil, fmt.Errorf("the server has http/2.0 enabled")
	}

	return transport, nil
}
func (d *testServerDependency) HostName() string { return d.server.URL }

type fakeCollector struct {
	t *testing.T
}

func (c fakeCollector) Collect(s backend.SampleResult) {
	if s.Sample == nil {
		c.t.Logf("all samples have been accounted for")
		return
	}
	c.t.Logf("(%s): %s", s.Sample, s.RequestResponse)
}

type fakeMonitor struct{}

func (fakeMonitor) StartInterval(t time.Time, condition monitorapi.Condition) int { return 0 }
func (fakeMonitor) EndInterval(startedInterval int, t time.Time)                  {}

// EventRecorder knows how to record events on behalf of an EventSource.
type fakeRecorder struct{}

func (fakeRecorder) Eventf(regarding runtime.Object, related runtime.Object, eventtype, reason, action, note string, args ...interface{}) {
}
