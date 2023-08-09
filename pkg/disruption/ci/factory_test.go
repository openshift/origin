package ci

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor"

	"github.com/openshift/origin/pkg/disruption/backend"
	"github.com/openshift/origin/pkg/monitor/monitorapi"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

func TestBackendSampler(t *testing.T) {
	var since int64
	ins := &instruction{}
	ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		shutdown := false
		elapsed := time.Duration(0)
		code := http.StatusOK
		switch ins.value() {
		case 0:
			// normal
		case 1:
			// disruption
			code = http.StatusInternalServerError
		case 2:
			// shutdown
			elapsed = time.Duration(atomic.AddInt64(&since, int64(time.Second)))
			shutdown = true
		}

		w.Header().Set("X-OpenShift-Disruption",
			fmt.Sprintf("shutdown=%t shutdown-delay-duration=10s elapsed=%s host=foo", shutdown, elapsed.Round(time.Second).String()))
		w.WriteHeader(code)
	}))
	ts.EnableHTTP2 = true
	ts.StartTLS()
	defer ts.Close()

	factory := &testFactory{
		dependency: &testServerDependency{
			server: ts,
		},
	}

	bs1, err := factory.New(TestConfiguration{
		TestDescriptor: TestDescriptor{
			TargetServer:     "test-server",
			LoadBalancerType: backend.ExternalLoadBalancerType,
			ConnectionType:   monitorapi.ReusedConnectionType,
			Protocol:         backend.ProtocolHTTP2,
		},
		Path:                         "/healthz",
		Timeout:                      10 * time.Second,
		SampleInterval:               time.Second,
		EnableShutdownResponseHeader: true,
	})
	if err != nil {
		t.Fatalf("failed to build backend sampler: %v", err)
	}

	bs2, err := factory.New(TestConfiguration{
		TestDescriptor: TestDescriptor{
			TargetServer:     "test-server",
			LoadBalancerType: backend.ExternalLoadBalancerType,
			ConnectionType:   monitorapi.NewConnectionType,
			Protocol:         backend.ProtocolHTTP2,
		},
		Path:                         "/healthz",
		Timeout:                      10 * time.Second,
		SampleInterval:               time.Second,
		EnableShutdownResponseHeader: true,
	})
	if err != nil {
		t.Fatalf("failed to build backend sampler: %v", err)
	}

	parent, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitorErrCh1, monitorErrCh2 := make(chan error, 1), make(chan error, 1)
	go func() {
		err := bs1.RunEndpointMonitoring(parent, monitor.NewRecorder(), &fakeRecorder{})
		monitorErrCh1 <- err
	}()
	go func() {
		err := bs2.RunEndpointMonitoring(parent, monitor.NewRecorder(), &fakeRecorder{})
		monitorErrCh2 <- err
	}()

	stopCh := make(chan struct{})
	go func() {
		defer close(stopCh)
		<-time.After(5 * time.Second)
		ins.set(1)
		<-time.After(5 * time.Second)
		ins.set(2)
		<-time.After(5 * time.Second)
		ins.set(0)
		<-time.After(15 * time.Second)
	}()
	go func() {
		<-stopCh
		bs1.Stop()
	}()
	go func() {
		<-stopCh
		bs2.Stop()
	}()

	t.Logf("waiting for monitor to be done")
	<-monitorErrCh1
	<-monitorErrCh2
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
	if tc.Protocol == backend.ProtocolHTTP1 && d.server.EnableHTTP2 {
		return nil, fmt.Errorf("the server has http/2.0 enabled")
	}

	return transport, nil
}
func (d *testServerDependency) HostName() string { return d.server.URL }
func (d *testServerDependency) GetHostNameDecoder() (backend.HostNameDecoderWithRunner, error) {
	return nil, nil
}
func (d *testServerDependency) GetRestConfig() *rest.Config { return nil }

type instruction struct {
	val int64
}

func (i *instruction) value() int64 {
	return atomic.LoadInt64(&i.val)
}
func (i *instruction) set(v int64) {
	atomic.StoreInt64(&i.val, v)
}

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

// EventRecorder knows how to record events on behalf of an EventSource.
type fakeRecorder struct {
}

func (fakeRecorder) Eventf(regarding runtime.Object, related runtime.Object, eventtype, reason, action, note string, args ...interface{}) {
}
