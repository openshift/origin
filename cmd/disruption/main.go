package main

import (
	"context"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/openshift/origin/pkg/disruption/backend"
	"github.com/openshift/origin/pkg/disruption/ci"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func main() {
	restConfig, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		klog.ErrorS(err, "failed to load config file")
		os.Exit(1)
	}

	factory := ci.NewDisruptionTestFactory(restConfig)
	backend1, err := factory.New(ci.TestConfiguration{
		TestDescriptor: ci.TestDescriptor{
			TargetServer:     ci.KubeAPIServer,
			LoadBalancerType: backend.ExternalLoadBalancerType,
			ConnectionType:   monitorapi.NewConnectionType,
			Protocol:         backend.ProtocolHTTP2,
		},
		Path:                         "/api/v1/namespaces/default",
		Timeout:                      10 * time.Second,
		EnableShutdownResponseHeader: true,
		SampleInterval:               time.Second,
	})
	if err != nil {
		klog.ErrorS(err, "failed to create disruption test")
		os.Exit(1)
	}

	backend2, err := factory.New(ci.TestConfiguration{
		TestDescriptor: ci.TestDescriptor{
			TargetServer:     ci.KubeAPIServer,
			LoadBalancerType: backend.ExternalLoadBalancerType,
			ConnectionType:   monitorapi.ReusedConnectionType,
			Protocol:         backend.ProtocolHTTP2,
		},
		Path:                         "/api/v1/namespaces/default",
		Timeout:                      10 * time.Second,
		EnableShutdownResponseHeader: true,
		SampleInterval:               time.Second,
	})
	if err != nil {
		klog.ErrorS(err, "failed to create disruption test")
		os.Exit(1)
	}

	parent, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitorErrCh1, monitorErrCh2 := make(chan error, 1), make(chan error, 1)
	go func() {
		err := backend1.RunEndpointMonitoring(parent, &recorder{}, &fakeRecorder{})
		monitorErrCh1 <- err
	}()
	go func() {
		err := backend2.RunEndpointMonitoring(parent, &recorder{}, &fakeRecorder{})
		monitorErrCh2 <- err
	}()

	stopCh := make(chan struct{})
	go func() {
		defer close(stopCh)
		<-time.After(1 * time.Hour)
	}()
	go func() {
		<-stopCh
		backend1.Stop()
	}()
	go func() {
		<-stopCh
		backend2.Stop()
	}()

	klog.Infof("waiting for monitor to be done")
	if err := <-monitorErrCh1; err != nil {
		klog.ErrorS(err, "disruption test returned an error")
	}
	if err := <-monitorErrCh2; err != nil {
		klog.ErrorS(err, "disruption test returned an error")
	}
}

type recorder struct{}

func (r recorder) StartInterval(t time.Time, condition monitorapi.Condition) int { return 0 }
func (r recorder) EndInterval(startedInterval int, t time.Time)                  {}

// EventRecorder knows how to record events on behalf of an EventSource.
type fakeRecorder struct{}

func (f fakeRecorder) Eventf(regarding runtime.Object, related runtime.Object, eventtype, reason, action, note string, args ...interface{}) {
}
