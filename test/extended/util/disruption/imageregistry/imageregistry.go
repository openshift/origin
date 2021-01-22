package imageregistry

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/onsi/ginkgo"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/events"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"

	routev1 "github.com/openshift/api/route/v1"
	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/test/extended/util/disruption"
)

// AvailableTest tests that the image registry is available before, during, and
// after a cluster upgrade.
type AvailableTest struct {
	routeRef *corev1.ObjectReference
	host     string
}

func (AvailableTest) Name() string { return "image-registry-available" }
func (AvailableTest) DisplayName() string {
	return "[sig-imageregistry] Image registry remain available"
}

// Setup creates a route that exposes the registry to tests.
func (t *AvailableTest) Setup(f *framework.Framework) {
	ctx := context.Background()

	config, err := framework.LoadConfig()
	framework.ExpectNoError(err)
	routeClient, err := routeclient.NewForConfig(config)
	framework.ExpectNoError(err)

	route, err := routeClient.RouteV1().Routes("openshift-image-registry").Create(ctx, &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-disruption",
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: "image-registry",
			},
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromInt(5000),
			},
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationPassthrough,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			},
		},
	}, metav1.CreateOptions{})
	framework.ExpectNoError(err)

	err = wait.PollImmediate(1*time.Second, 30*time.Second, func() (bool, error) {
		route, err = routeClient.RouteV1().Routes("openshift-image-registry").Get(ctx, "test-disruption", metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		for _, ingress := range route.Status.Ingress {
			if len(ingress.Host) > 0 {
				t.host = ingress.Host
				return true, nil
			}
		}
		return false, nil
	})
	framework.ExpectNoError(err, "failed to get route host")

	t.routeRef = &corev1.ObjectReference{
		Kind:      "Route",
		Namespace: route.Namespace,
		Name:      route.Name,
	}
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
	err = t.startEndpointMonitoring(ctx, m, r)
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
	ctx := context.Background()

	config, err := framework.LoadConfig()
	framework.ExpectNoError(err)
	routeClient, err := routeclient.NewForConfig(config)
	framework.ExpectNoError(err)

	err = routeClient.RouteV1().Routes(t.routeRef.Namespace).Delete(ctx, t.routeRef.Name, metav1.DeleteOptions{})
	framework.ExpectNoError(err, "failed to delete route")
}

func (t *AvailableTest) startEndpointMonitoring(ctx context.Context, m *monitor.Monitor, r events.EventRecorder) error {
	var (
		url     = "https://" + t.host
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
					Message: "Route started responding to GET requests on reused connections",
				}
			case err != nil && previous:
				framework.Logf("Route for image-registry is unreachable on reused connections: %v", err)
				r.Eventf(t.routeRef, nil, corev1.EventTypeWarning, "Unreachable", "detected", "on reused connections")
				condition = &monitor.Condition{
					Level:   monitor.Error,
					Locator: locator,
					Message: "Route stopped responding to GET requests on reused connections",
				}
			case err != nil:
				framework.Logf("Route for image-registry is unreachable on reused connections: %v", err)
			}
			return condition, err == nil
		}).ConditionWhenFailing(&monitor.Condition{
			Level:   monitor.Error,
			Locator: locator,
			Message: "Route is not responding to GET requests on reused connections",
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
					Message: "Route started responding to GET requests over new connections",
				}
			case err != nil && previous:
				framework.Logf("Route for image-registry is unreachable on new connections: %v", err)
				r.Eventf(t.routeRef, nil, corev1.EventTypeWarning, "Unreachable", "detected", "on new connections")
				condition = &monitor.Condition{
					Level:   monitor.Error,
					Locator: locator,
					Message: "Route stopped responding to GET requests over new connections",
				}
			case err != nil:
				framework.Logf("Route for image-registry is unreachable on new connections: %v", err)
			}
			return condition, err == nil
		}).ConditionWhenFailing(&monitor.Condition{
			Level:   monitor.Error,
			Locator: locator,
			Message: "Route is not responding to GET requests over new connections",
		}),
	)

	return nil
}
