package frontends

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"time"

	"github.com/onsi/ginkgo"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/events"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"

	routeclientset "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/test/extended/util/disruption"
)

// AvailableTest tests that route frontends are available before, during, and
// after a cluster upgrade.
type AvailableTest struct {
	frontends []Frontend
}

type Frontend struct {
	Namespace string
	Name      string
	URL       string
	Path      string

	Expect       string
	ExpectRegexp *regexp.Regexp
}

func (AvailableTest) Name() string { return "frontend-ingress-available" }
func (AvailableTest) DisplayName() string {
	return "[sig-network-edge] Cluster frontend ingress remain available"
}

// Setup finds the routes the platform exposes by default
func (t *AvailableTest) Setup(f *framework.Framework) {
	t.frontends = []Frontend{
		{Namespace: "openshift-authentication", Name: "oauth-openshift", Path: "/healthz", Expect: "ok"},
		{Namespace: "openshift-console", Name: "console", ExpectRegexp: regexp.MustCompile(`(Red Hat OpenShift Container Platform|OKD)`)},
	}
	config, err := framework.LoadConfig()
	framework.ExpectNoError(err)
	client, err := routeclientset.NewForConfig(config)
	framework.ExpectNoError(err)
	for i, frontend := range t.frontends {
		route, err := client.RouteV1().Routes(frontend.Namespace).Get(context.Background(), frontend.Name, metav1.GetOptions{})
		framework.ExpectNoError(err)
		for _, ingress := range route.Status.Ingress {
			if len(ingress.Host) > 0 {
				t.frontends[i].URL = fmt.Sprintf("https://%s", ingress.Host)

				break
			}
		}
		if len(t.frontends[i].URL) == 0 {
			framework.Failf("route %s/%s has no ingress host: %#v", route.Namespace, route.Name, route.Status.Ingress)
		}
	}
}

// Test runs a connectivity check to the service.
func (t *AvailableTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	client, err := framework.LoadClientset()
	framework.ExpectNoError(err)

	stopCh := make(chan struct{})
	defer close(stopCh)
	newBroadcaster := events.NewBroadcaster(&events.EventSinkImpl{Interface: client.EventsV1beta1().Events("")})
	r := newBroadcaster.NewRecorder(scheme.Scheme, "openshift.io/frontends-available-test")
	newBroadcaster.StartRecordingToSink(stopCh)

	ginkgo.By("continuously hitting infrastructure through the router")

	ctx, cancel := context.WithCancel(context.Background())
	m := monitor.NewMonitorWithInterval(1 * time.Second)
	for _, frontend := range t.frontends {
		err = startEndpointMonitoring(ctx, m, frontend, r)
		framework.ExpectNoError(err, "unable to monitor route")
	}

	start := time.Now()
	m.StartSampling(ctx)

	// wait to ensure API is still up after the test ends
	<-done
	ginkgo.By("waiting for any post disruption failures")
	time.Sleep(15 * time.Second)
	cancel()
	end := time.Now()

	disruption.ExpectNoDisruption(f, 0.20, end.Sub(start), m.Events(time.Time{}, time.Time{}), "Frontends were unreachable during disruption")
}

// Teardown cleans up any remaining resources.
func (t *AvailableTest) Teardown(f *framework.Framework) {
	// rely on the namespace deletion to clean up everything
}

func startEndpointMonitoring(ctx context.Context, m *monitor.Monitor, frontend Frontend, r events.EventRecorder) error {
	// this client reuses connections and detects abrupt breaks
	continuousClient, err := rest.UnversionedRESTClientFor(&rest.Config{
		Host:    frontend.URL,
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
			data, err := continuousClient.Get().AbsPath(frontend.Path).DoRaw(ctx)
			if err == nil && !bytes.Contains(data, []byte(frontend.Expect)) {
				err = fmt.Errorf("route returned success but did not contain the correct body contents: %q", string(data))
			}
			switch {
			case err == nil && !previous:
				condition = &monitor.Condition{
					Level:   monitor.Info,
					Locator: locateRoute(frontend.Namespace, frontend.Name),
					Message: "Route started responding to GET requests on reused connections",
				}
			case err != nil && previous:
				framework.Logf("Route %s is unreachable on reused connections: %v", frontend.Name, err)
				r.Eventf(&v1.ObjectReference{Kind: "Route", Namespace: "kube-system", Name: frontend.Name}, nil, v1.EventTypeWarning, "Unreachable", "detected", "on reused connections")
				condition = &monitor.Condition{
					Level:   monitor.Error,
					Locator: locateRoute(frontend.Namespace, frontend.Name),
					Message: "Route stopped responding to GET requests on reused connections",
				}
			case err != nil:
				framework.Logf("Route %s is unreachable on reused connections: %v", frontend.Name, err)
			}
			return condition, err == nil
		}).ConditionWhenFailing(&monitor.Condition{
			Level:   monitor.Error,
			Locator: locateRoute(frontend.Namespace, frontend.Name),
			Message: "Route is not responding to GET requests on reused connections",
		}),
	)

	// this client creates fresh connections and detects failure to establish connections
	client, err := rest.UnversionedRESTClientFor(&rest.Config{
		Host:    frontend.URL,
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
			data, err := client.Get().AbsPath(frontend.Path).DoRaw(ctx)
			switch {
			case err == nil && len(frontend.Expect) != 0 && !bytes.Contains(data, []byte(frontend.Expect)):
				err = fmt.Errorf("route returned success but did not contain the correct body contents: %q", string(data))
			case err == nil && frontend.ExpectRegexp != nil && !frontend.ExpectRegexp.MatchString(string(data)):
				err = fmt.Errorf("route returned success but did not contain the correct body contents: %q", string(data))
			}
			switch {
			case err == nil && !previous:
				condition = &monitor.Condition{
					Level:   monitor.Info,
					Locator: locateRoute(frontend.Namespace, frontend.Name),
					Message: "Route started responding to GET requests over new connections",
				}
			case err != nil && previous:
				framework.Logf("Route %s is unreachable on new connections: %v", frontend.Name, err)
				r.Eventf(&v1.ObjectReference{Kind: "Route", Namespace: "kube-system", Name: frontend.Name}, nil, v1.EventTypeWarning, "Unreachable", "detected", "on new connections")
				condition = &monitor.Condition{
					Level:   monitor.Error,
					Locator: locateRoute(frontend.Namespace, frontend.Name),
					Message: "Route stopped responding to GET requests over new connections",
				}
			case err != nil:
				framework.Logf("Route %s is unreachable on new connections: %v", frontend.Name, err)
			}
			return condition, err == nil
		}).ConditionWhenFailing(&monitor.Condition{
			Level:   monitor.Error,
			Locator: locateRoute(frontend.Namespace, frontend.Name),
			Message: "Route is not responding to GET requests over new connections",
		}),
	)
	return nil
}

func locateRoute(ns, name string) string {
	return fmt.Sprintf("ns/%s route/%s", ns, name)
}
