package imageregistry

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/onsi/ginkgo"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/events"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/test/extended/util/disruption"
)

// AvailableTest tests that the image registry is available before, during, and
// after a cluster upgrade.
type AvailableTest struct {
}

func (AvailableTest) Name() string { return "image-registry-available" }
func (AvailableTest) DisplayName() string {
	return "[sig-imageregistry] Image registry remain available"
}

// Setup does nothing.
func (t *AvailableTest) Setup(f *framework.Framework) {
}

// Test runs a connectivity check to the service.
func (t *AvailableTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	client, err := framework.LoadClientset()
	framework.ExpectNoError(err)

	stopCh := make(chan struct{})
	defer close(stopCh)

	newBroadcaster := events.NewBroadcaster(&events.EventSinkImpl{Interface: client.EventsV1()})
	r := newBroadcaster.NewRecorder(scheme.Scheme, "openshift.io/image-registry-available-test")
	newBroadcaster.StartRecordingToSink(stopCh)

	ginkgo.By("continuously hitting image registry")

	ctx, cancel := context.WithCancel(context.Background())
	m := monitor.NewMonitorWithInterval(1 * time.Second)
	err = startEndpointMonitoring(ctx, m, r)
	framework.ExpectNoError(err, "unable to monitor route")

	start := time.Now()
	m.StartSampling(ctx)

	// wait to ensure API is still up after the test ends
	<-done
	ginkgo.By("waiting for any post disruption failures")
	time.Sleep(15 * time.Second)
	cancel()
	end := time.Now()

	disruption.ExpectNoDisruption(f, 0.20, end.Sub(start), m.Events(time.Time{}, time.Time{}), "Image registry was unreachable during disruption")
}

// Teardown cleans up any remaining resources.
func (t *AvailableTest) Teardown(f *framework.Framework) {
	// rely on the namespace deletion to clean up everything
}

func startEndpointMonitoring(ctx context.Context, m *monitor.Monitor, r events.EventRecorder) error {
	const (
		url     = "https://image-registry.openshift-image-registry.svc:5000"
		path    = "/healthz"
		locator = "image-registry"
	)

	// this client reuses connections and detects abrupt breaks
	continuousClient, err := rest.UnversionedRESTClientFor(&rest.Config{
		Host:    url,
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 15 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 15 * time.Second,
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		},
		ContentConfig: rest.ContentConfig{
			NegotiatedSerializer: runtime.NewSimpleNegotiatedSerializer(scheme.Codecs.SupportedMediaTypes()[0]),
		},
	})
	if err != nil {
		return err
	}
	m.AddSampler(
		monitor.StartSampling(ctx, m, time.Second, func(previous bool) (condition *monitor.Condition, next bool) {
			_, err := continuousClient.Get().AbsPath(path).DoRaw(ctx)
			switch {
			case err == nil && !previous:
				condition = &monitor.Condition{
					Level:   monitor.Info,
					Locator: locator,
					Message: "Service started responding to GET requests on reused connections",
				}
			case err != nil && previous:
				framework.Logf("Service image-registry is unreachable on reused connections: %v", err)
				r.Eventf(&v1.ObjectReference{Kind: "Service", Namespace: "openshift-image-registry", Name: "image-registry"}, nil, v1.EventTypeWarning, "Unreachable", "detected", "on reused connections")
				condition = &monitor.Condition{
					Level:   monitor.Error,
					Locator: locator,
					Message: "Service stopped responding to GET requests on reused connections",
				}
			case err != nil:
				framework.Logf("Service image-registry is unreachable on reused connections: %v", err)
			}
			return condition, err == nil
		}).ConditionWhenFailing(&monitor.Condition{
			Level:   monitor.Error,
			Locator: locator,
			Message: "Service is not responding to GET requests on reused connections",
		}),
	)

	// this client creates fresh connections and detects failure to establish connections
	client, err := rest.UnversionedRESTClientFor(&rest.Config{
		Host:    url,
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   15 * time.Second,
				KeepAlive: -1,
			}).Dial,
			TLSHandshakeTimeout: 15 * time.Second,
			IdleConnTimeout:     15 * time.Second,
			DisableKeepAlives:   true,
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		},
		ContentConfig: rest.ContentConfig{
			NegotiatedSerializer: runtime.NewSimpleNegotiatedSerializer(scheme.Codecs.SupportedMediaTypes()[0]),
		},
	})
	if err != nil {
		return err
	}
	m.AddSampler(
		monitor.StartSampling(ctx, m, time.Second, func(previous bool) (condition *monitor.Condition, next bool) {
			_, err := client.Get().AbsPath(path).DoRaw(ctx)
			switch {
			case err == nil && !previous:
				condition = &monitor.Condition{
					Level:   monitor.Info,
					Locator: locator,
					Message: "Service started responding to GET requests over new connections",
				}
			case err != nil && previous:
				framework.Logf("Service image-registry is unreachable on new connections: %v", err)
				r.Eventf(&v1.ObjectReference{Kind: "Service", Namespace: "openshift-image-registry", Name: "image-registry"}, nil, v1.EventTypeWarning, "Unreachable", "detected", "on new connections")
				condition = &monitor.Condition{
					Level:   monitor.Error,
					Locator: locator,
					Message: "Service stopped responding to GET requests over new connections",
				}
			case err != nil:
				framework.Logf("Service image-registry is unreachable on new connections: %v", err)
			}
			return condition, err == nil
		}).ConditionWhenFailing(&monitor.Condition{
			Level:   monitor.Error,
			Locator: locator,
			Message: "Service is not responding to GET requests over new connections",
		}),
	)

	return nil
}
