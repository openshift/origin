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

	"github.com/blang/semver"
	"github.com/openshift/origin/pkg/monitor/monitorapi"

	"github.com/onsi/ginkgo"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/events"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"

	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	routeclientset "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/disruption"
)

type Frontend struct {
	Namespace string
	Name      string
	URL       string
	Path      string

	Expect       string
	ExpectRegexp *regexp.Regexp
}

var (
	oauthRouteFrontend = Frontend{
		Namespace: "openshift-authentication",
		Name:      "oauth-openshift",
		Path:      "/healthz",
		Expect:    "ok",
	}
	consoleRouteFrontend = Frontend{
		Namespace:    "openshift-console",
		Name:         "console",
		ExpectRegexp: regexp.MustCompile(`(Red Hat OpenShift Container Platform|OKD)`),
	}
)

// NewOAuthRouteAvailableWithNewConnectionsTest tests that the oauth route
// remains available during and after a cluster upgrade, using a new connection
// for each request.
func NewOAuthRouteAvailableWithNewConnectionsTest() upgrades.Test {
	return &availableTest{
		testName:        "[sig-network-edge] OAuth remains available via cluster frontend ingress using new connections",
		name:            "frontend-ingress-oauth-available-new",
		frontend:        oauthRouteFrontend,
		startMonitoring: startEndpointMonitoringWithNewConnections,
	}
}

// NewOAuthRouteAvailableWithConnectionReuseTest tests that the oauth route
// remains available during and after a cluster upgrade, reusing a connection
// for requests.
func NewOAuthRouteAvailableWithConnectionReuseTest() upgrades.Test {
	return &availableTest{
		testName:        "[sig-network-edge] OAuth remains available via cluster frontend ingress using reused connections",
		name:            "frontend-ingress-oauth-available-reuse",
		frontend:        oauthRouteFrontend,
		startMonitoring: startEndpointMonitoringWithConnectionReuse,
	}
}

// NewConsoleRouteAvailableWithNewConnectionsTest tests that the console route
// remains available during and after a cluster upgrade, using a new connection
// for each request.
func NewConsoleRouteAvailableWithNewConnectionsTest() upgrades.Test {
	return &availableTest{
		testName:        "[sig-network-edge] Console remains available via cluster frontend ingress using new connections",
		name:            "frontend-ingress-console-available-new",
		frontend:        consoleRouteFrontend,
		startMonitoring: startEndpointMonitoringWithNewConnections,
	}
}

// NewConsoleRouteAvailableWithConnectionReuseTest tests that the console route
// remains available during and after a cluster upgrade, reusing a connection
// for requests.
func NewConsoleRouteAvailableWithConnectionReuseTest() upgrades.Test {
	return &availableTest{
		testName:        "[sig-network-edge] Console remains available via cluster frontend ingress using reused connections",
		name:            "frontend-ingress-console-available-reuse",
		frontend:        consoleRouteFrontend,
		startMonitoring: startEndpointMonitoringWithConnectionReuse,
	}
}

// availableTest tests that route frontends are available before, during, and
// after a cluster upgrade.
type availableTest struct {
	// testName is the name to show in unit.
	testName string
	// name helps distinguish which route is unavailable.
	name string
	// frontend describes a route that should be monitored.
	frontend Frontend
	// startMonitoring is a function that starts the monitor.
	startMonitoring starter
}

type starter func(ctx context.Context, m *monitor.Monitor, frontend Frontend, r events.EventRecorder) error

func (t *availableTest) Name() string { return t.name }
func (t *availableTest) DisplayName() string {
	return t.testName
}

// Setup looks up the host of the route specified by the frontend and updates
// the frontend with the route's host.
func (t *availableTest) Setup(f *framework.Framework) {
	// Setup may have already been called for a different availability test
	// that uses the same frontend.  In that case, we needn't repeat setup.
	if len(t.frontend.URL) != 0 {
		return
	}
	config, err := framework.LoadConfig()
	framework.ExpectNoError(err)
	client, err := routeclientset.NewForConfig(config)
	framework.ExpectNoError(err)
	route, err := client.RouteV1().Routes(t.frontend.Namespace).Get(context.Background(), t.frontend.Name, metav1.GetOptions{})
	framework.ExpectNoError(err)
	for _, ingress := range route.Status.Ingress {
		if len(ingress.Host) > 0 {
			t.frontend.URL = fmt.Sprintf("https://%s", ingress.Host)

			break
		}
	}
	if len(t.frontend.URL) == 0 {
		framework.Failf("route %s/%s has no ingress host: %#v", route.Namespace, route.Name, route.Status.Ingress)
	}
}

// Test runs a connectivity check to a route.
func (t *availableTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	config, err := framework.LoadConfig()
	framework.ExpectNoError(err)
	client, err := framework.LoadClientset()
	framework.ExpectNoError(err)

	stopCh := make(chan struct{})
	defer close(stopCh)
	newBroadcaster := events.NewBroadcaster(&events.EventSinkImpl{Interface: client.EventsV1()})
	r := newBroadcaster.NewRecorder(scheme.Scheme, "openshift.io/"+t.name)
	newBroadcaster.StartRecordingToSink(stopCh)

	ginkgo.By("continuously hitting infrastructure through the router")

	ctx, cancel := context.WithCancel(context.Background())
	m := monitor.NewMonitorWithInterval(1 * time.Second)
	err = t.startMonitoring(ctx, m, t.frontend, r)
	framework.ExpectNoError(err, "unable to monitor route")

	start := time.Now()
	m.StartSampling(ctx)

	// Wait to ensure the route is still available after the test ends.
	<-done
	ginkgo.By("waiting for any post disruption failures")
	time.Sleep(15 * time.Second)
	cancel()
	end := time.Now()

	// starting from 4.8, enforce the requirement that frontends remains available
	hasAllFixes, err := util.AllClusterVersionsAreGTE(semver.Version{Major: 4, Minor: 8}, config)
	if err != nil {
		framework.Logf("Cannot require full cluster ingress frontend availability; some versions could not be checked: %v", err)
	}

	// Fetch network type for considering whether we allow disruption. For OVN, we currently have to allow disruption
	// as those tests are failing: BZ: https://bugzilla.redhat.com/show_bug.cgi?id=1983829
	c, err := configv1client.NewForConfig(config)
	framework.ExpectNoError(err)
	network, err := c.ConfigV1().Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
	framework.ExpectNoError(err)

	toleratedDisruption := 0.20
	switch {
	case network.Status.NetworkType == "OVNKubernetes":
		framework.Logf("Network type is OVNKubernetes, temporarily allowing disruption due to BZ https://bugzilla.redhat.com/show_bug.cgi?id=1983829")
	// framework.ProviderIs("gce") removed here in 4.9 due to regression. BZ: https://bugzilla.redhat.com/show_bug.cgi?id=1983758
	case framework.ProviderIs("azure"), framework.ProviderIs("aws"):
		if hasAllFixes {
			framework.Logf("Cluster contains no versions older than 4.8, tolerating no disruption")
			toleratedDisruption = 0
		}
	}
	disruption.ExpectNoDisruption(f, toleratedDisruption, end.Sub(start), m.Intervals(time.Time{}, time.Time{}), "Frontend was unreachable during disruption")
}

// Teardown cleans up any remaining resources.
func (t *availableTest) Teardown(f *framework.Framework) {
	// rely on the namespace deletion to clean up everything
}

// startEndpointMonitoring sets up a client for the given frontend and starts a
// new sampler for the given monitor that uses the client to monitor
// connectivity to the frontend and reports any observed disruption.
//
// If disableConnectionReuse is false, the client reuses connections and detects
// abrupt breaks in connectivity.  If disableConnectionReuse is true, the client
// instead creates fresh connections so that it detects failures to establish
// connections.
func startEndpointMonitoring(ctx context.Context, m *monitor.Monitor, frontend Frontend, r events.EventRecorder, disableConnectionReuse bool) error {
	var keepAlive time.Duration
	connectionType := "reused"
	if disableConnectionReuse {
		keepAlive = -1
		connectionType = "new"
	}
	client, err := rest.UnversionedRESTClientFor(&rest.Config{
		Host:    frontend.URL,
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   15 * time.Second,
				KeepAlive: keepAlive,
			}).Dial,
			TLSHandshakeTimeout: 15 * time.Second,
			IdleConnTimeout:     15 * time.Second,
			DisableKeepAlives:   disableConnectionReuse,
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		},
		ContentConfig: rest.ContentConfig{
			NegotiatedSerializer: runtime.NewSimpleNegotiatedSerializer(scheme.Codecs.SupportedMediaTypes()[0]),
		},
	})
	if err != nil {
		return err
	}
	go monitor.NewSampler(m, time.Second, func(previous bool) (condition *monitorapi.Condition, next bool) {
		data, err := client.Get().AbsPath(frontend.Path).DoRaw(ctx)
		switch {
		case err == nil && len(frontend.Expect) != 0 && !bytes.Contains(data, []byte(frontend.Expect)):
			err = fmt.Errorf("route returned success but did not contain the correct body contents: %q", string(data))
		case err == nil && frontend.ExpectRegexp != nil && !frontend.ExpectRegexp.MatchString(string(data)):
			err = fmt.Errorf("route returned success but did not contain the correct body contents: %q", string(data))
		}
		switch {
		case err == nil && !previous:
			condition = &monitorapi.Condition{
				Level:   monitorapi.Info,
				Locator: locateRoute(frontend.Namespace, frontend.Name),
				Message: fmt.Sprintf("Route started responding to GET requests on %s connections", connectionType),
			}
		case err != nil && previous:
			framework.Logf("Route %s is unreachable on %s connections: %v", frontend.Name, connectionType, err)
			r.Eventf(&v1.ObjectReference{Kind: "Route", Namespace: frontend.Namespace, Name: frontend.Name}, nil, v1.EventTypeWarning, "Unreachable", "detected", fmt.Sprintf("on %s connections", connectionType))
			condition = &monitorapi.Condition{
				Level:   monitorapi.Error,
				Locator: locateRoute(frontend.Namespace, frontend.Name),
				Message: fmt.Sprintf("Route stopped responding to GET requests on %s connections", connectionType),
			}
		case err != nil:
			framework.Logf("Route %s is unreachable on %s connections: %v", frontend.Name, connectionType, err)
		}
		return condition, err == nil
	}).WhenFailing(ctx, &monitorapi.Condition{
		Level:   monitorapi.Error,
		Locator: locateRoute(frontend.Namespace, frontend.Name),
		Message: fmt.Sprintf("Route is not responding to GET requests on %s connections", connectionType),
	})
	return nil
}

func locateRoute(ns, name string) string {
	return fmt.Sprintf("ns/%s route/%s", ns, name)
}

func startEndpointMonitoringWithNewConnections(ctx context.Context, m *monitor.Monitor, frontend Frontend, r events.EventRecorder) error {
	return startEndpointMonitoring(ctx, m, frontend, r, true)
}

func startEndpointMonitoringWithConnectionReuse(ctx context.Context, m *monitor.Monitor, frontend Frontend, r events.EventRecorder) error {
	return startEndpointMonitoring(ctx, m, frontend, r, false)
}
