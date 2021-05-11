package monitor

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	configclientset "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/monitor/intervalcreation"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	kubeinformers "k8s.io/client-go/informers"
	informersv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	listersv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/transport"
)

const (
	LocatorKubeAPIServerNewConnection          = "kube-apiserver-new-connection"
	LocatorKubeAPIServerNewConnectionNodeIP    = "kube-apiserver-new-connection-node-ips"
	LocatorOpenshiftAPIServerNewConnection     = "openshift-apiserver-new-connection"
	LocatorOAuthAPIServerNewConnection         = "oauth-apiserver-new-connection"
	LocatorKubeAPIServerReusedConnection       = "kube-apiserver-reused-connection"
	LocatorKubeAPIServerReusedConnectionNodeIP = "kube-apiserver-reused-connection-node-ips"
	LocatorOpenshiftAPIServerReusedConnection  = "openshift-apiserver-reused-connection"
	LocatorOAuthAPIServerReusedConnection      = "oauth-apiserver-reused-connection"
)

// Start begins monitoring the cluster referenced by the default kube configuration until
// context is finished.
func Start(ctx context.Context) (*Monitor, error) {
	m := NewMonitorWithInterval(time.Second)
	cfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{})
	clusterConfig, err := cfg.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("could not load client configuration: %v", err)
	}
	client, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		return nil, err
	}
	configClient, err := configclientset.NewForConfig(clusterConfig)
	if err != nil {
		return nil, err
	}

	if err := StartKubeAPIMonitoringWithNewConnections(ctx, m, clusterConfig, 5*time.Second); err != nil {
		return nil, err
	}
	if err := StartKubeAPIMonitoringWithNewConnectionsForNodeIPs(ctx, m, clusterConfig, 5*time.Second); err != nil {
		return nil, err
	}
	if err := StartOpenShiftAPIMonitoringWithNewConnections(ctx, m, clusterConfig, 5*time.Second); err != nil {
		return nil, err
	}
	if err := StartOAuthAPIMonitoringWithNewConnections(ctx, m, clusterConfig, 5*time.Second); err != nil {
		return nil, err
	}
	if err := StartKubeAPIMonitoringWithConnectionReuse(ctx, m, clusterConfig, 5*time.Second); err != nil {
		return nil, err
	}
	if err := StartKubeAPIMonitoringWithConnectionReuseForNodeIPs(ctx, m, clusterConfig, 5*time.Second); err != nil {
		return nil, err
	}
	if err := StartOpenShiftAPIMonitoringWithConnectionReuse(ctx, m, clusterConfig, 5*time.Second); err != nil {
		return nil, err
	}
	if err := StartOAuthAPIMonitoringWithConnectionReuse(ctx, m, clusterConfig, 5*time.Second); err != nil {
		return nil, err
	}
	startPodMonitoring(ctx, m, client)
	startNodeMonitoring(ctx, m, client)
	startEventMonitoring(ctx, m, client)

	// add interval creation at the same point where we add the monitors
	startClusterOperatorMonitoring(ctx, m, configClient)
	m.intervalCreationFns = append(
		m.intervalCreationFns,
		intervalcreation.IntervalsFromEvents_OperatorAvailable,
		intervalcreation.IntervalsFromEvents_OperatorProgressing,
		intervalcreation.IntervalsFromEvents_OperatorDegraded,
		intervalcreation.IntervalsFromEvents_E2ETests,
		intervalcreation.IntervalsFromEvents_NodeChanges,
	)

	m.StartSampling(ctx)
	return m, nil
}

func StartServerMonitoringWithNewConnections(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration, resourceLocator, url string) error {
	return startServerMonitoring(ctx, m, clusterConfig, timeout, resourceLocator, url, true)
}

func StartServerMonitoringWithConnectionReuse(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration, resourceLocator, url string) error {
	return startServerMonitoring(ctx, m, clusterConfig, timeout, resourceLocator, url, false)
}

func startServerMonitoring(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration, resourceLocator, url string, disableConnectionReuse bool) error {
	kubeTransportConfig, err := clusterConfig.TransportConfig()
	if err != nil {
		return err
	}
	tlsConfig, err := transport.TLSConfigFor(kubeTransportConfig)
	if err != nil {
		return err
	}
	var httpTransport *http.Transport

	if disableConnectionReuse {
		httpTransport = &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   timeout,
				KeepAlive: -1, // this looks unnecessary to me, but it was set in other code.
			}).Dial,
			TLSClientConfig:     tlsConfig,
			TLSHandshakeTimeout: timeout,
			DisableKeepAlives:   true, // this prevents connections from being reused
			IdleConnTimeout:     timeout,
		}
	} else {
		httpTransport = &http.Transport{
			Dial: (&net.Dialer{
				Timeout: timeout,
			}).Dial,
			TLSClientConfig:     tlsConfig,
			TLSHandshakeTimeout: timeout,
			IdleConnTimeout:     timeout,
		}
	}

	roundTripper := http.RoundTripper(httpTransport)
	if kubeTransportConfig.HasTokenAuth() {
		roundTripper, err = transport.NewBearerAuthWithRefreshRoundTripper(kubeTransportConfig.BearerToken, kubeTransportConfig.BearerTokenFile, httpTransport)
		if err != nil {
			return err
		}

	}

	if kubeTransportConfig.WrapTransport != nil {
		roundTripper = kubeTransportConfig.WrapTransport(roundTripper)
	}

	httpClient := http.Client{
		Transport: roundTripper,
	}

	m.AddSampler(
		StartSampling(ctx, m, time.Second, func(previous bool) (condition *monitorapi.Condition, next bool) {
			resp, err := httpClient.Get(clusterConfig.Host + url)

			// we don't have an error, but the response code was an error, then we have to set an artificial error for the logic below to work.
			if err == nil && (resp.StatusCode < 200 || resp.StatusCode > 399) {
				body, err := ioutil.ReadAll(resp.Body)
				if err == nil {
					err = fmt.Errorf("error running request: %v: %v", resp.Status, string(body))
				} else {
					err = fmt.Errorf("error running request: %v", resp.Status)
				}
			}
			if resp != nil && resp.Body != nil {
				defer resp.Body.Close()
			}

			switch {
			case err == nil && !previous:
				condition = &monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: resourceLocator,
					Message: fmt.Sprintf("%s started responding to GET requests", resourceLocator),
				}
			case err != nil && previous:
				condition = &monitorapi.Condition{
					Level:   monitorapi.Error,
					Locator: resourceLocator,
					Message: fmt.Sprintf("%s started failing: %v", resourceLocator, err),
				}
			}
			return condition, err == nil
		}).ConditionWhenFailing(&monitorapi.Condition{
			Level:   monitorapi.Error,
			Locator: resourceLocator,
			Message: fmt.Sprintf("%s is not responding to GET requests", resourceLocator),
		}),
	)
	return nil
}

func StartKubeAPIMonitoringWithNewConnections(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration) error {
	// default gets auto-created, so this should always exist
	return StartServerMonitoringWithNewConnections(ctx, m, clusterConfig, timeout, LocatorKubeAPIServerNewConnection, "/api/v1/namespaces/default")
}

func StartKubeAPIMonitoringWithNewConnectionsForNodeIPs(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration) error {
	client, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		return err
	}
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, time.Second*30)
	endpointsInformer := kubeInformerFactory.Core().V1().Endpoints()

	newClusterConfig := kubeAPIMonitoringConfigForNodeIPs(endpointsInformer, clusterConfig)
	// default gets auto-created, so this should always exist
	if err := StartServerMonitoringWithNewConnections(ctx, m, newClusterConfig, timeout, LocatorKubeAPIServerNewConnectionNodeIP, "/api/v1/namespaces/default"); err != nil {
		return err
	}

	kubeInformerFactory.Start(ctx.Done())
	if ok := cache.WaitForCacheSync(ctx.Done(), endpointsInformer.Informer().HasSynced); !ok {
		return fmt.Errorf("failed to wait for the endpoints informer to sync")
	}

	return nil
}

func StartOpenShiftAPIMonitoringWithNewConnections(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration) error {
	// this request should never 404, but should be empty/small
	return StartServerMonitoringWithNewConnections(ctx, m, clusterConfig, timeout, LocatorOpenshiftAPIServerNewConnection, "/apis/image.openshift.io/v1/namespaces/default/imagestreams")
}

func StartOAuthAPIMonitoringWithNewConnections(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration) error {
	// this should be relatively small and should not ever 404
	return StartServerMonitoringWithNewConnections(ctx, m, clusterConfig, timeout, LocatorOAuthAPIServerNewConnection, "/apis/oauth.openshift.io/v1/oauthclients")
}

func StartKubeAPIMonitoringWithConnectionReuse(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration) error {
	// default gets auto-created, so this should always exist
	return StartServerMonitoringWithConnectionReuse(ctx, m, clusterConfig, timeout, LocatorKubeAPIServerReusedConnection, "/api/v1/namespaces/default")
}

func StartKubeAPIMonitoringWithConnectionReuseForNodeIPs(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration) error {
	client, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		return err
	}
	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(client, time.Second*30)
	endpointsInformer := kubeInformerFactory.Core().V1().Endpoints()

	newClusterConfig := kubeAPIMonitoringConfigForNodeIPs(endpointsInformer, clusterConfig)
	// default gets auto-created, so this should always exist
	if err := StartServerMonitoringWithConnectionReuse(ctx, m, newClusterConfig, timeout, LocatorKubeAPIServerReusedConnectionNodeIP, "/api/v1/namespaces/default"); err != nil {
		return err
	}

	kubeInformerFactory.Start(ctx.Done())
	if ok := cache.WaitForCacheSync(ctx.Done(), endpointsInformer.Informer().HasSynced); !ok {
		return fmt.Errorf("failed to wait for the endpoints informer to sync")
	}
	return nil
}

func StartOpenShiftAPIMonitoringWithConnectionReuse(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration) error {
	// this request should never 404, but should be empty/small
	return StartServerMonitoringWithConnectionReuse(ctx, m, clusterConfig, timeout, LocatorOpenshiftAPIServerReusedConnection, "/apis/image.openshift.io/v1/namespaces/default/imagestreams")
}

func StartOAuthAPIMonitoringWithConnectionReuse(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration) error {
	// this should be relatively small and should not ever 404
	return StartServerMonitoringWithConnectionReuse(ctx, m, clusterConfig, timeout, LocatorOAuthAPIServerReusedConnection, "/apis/oauth.openshift.io/v1/oauthclients")
}

func kubeAPIMonitoringConfigForNodeIPs(endpointsInformer informersv1.EndpointsInformer, clusterConfig *rest.Config) *rest.Config {
	clusterConfigCopy := *clusterConfig
	clusterConfigCopy.TLSClientConfig.ServerName = "kubernetes.default.svc"
	clusterConfigCopy.Wrap(newKubeAPIEndpointsAwareRoundTripper(endpointsInformer))
	return &clusterConfigCopy
}

func findContainerStatus(status []corev1.ContainerStatus, name string, position int) *corev1.ContainerStatus {
	if position < len(status) {
		if status[position].Name == name {
			return &status[position]
		}
	}
	for i := range status {
		if status[i].Name == name {
			return &status[i]
		}
	}
	return nil
}

func findNodeCondition(status []corev1.NodeCondition, name corev1.NodeConditionType, position int) *corev1.NodeCondition {
	if position < len(status) {
		if status[position].Type == name {
			return &status[position]
		}
	}
	for i := range status {
		if status[i].Type == name {
			return &status[i]
		}
	}
	return nil
}

func locateEvent(event *corev1.Event) string {
	if len(event.InvolvedObject.Namespace) > 0 {
		if len(event.Source.Host) > 0 && event.InvolvedObject.Kind != "Node" {
			return fmt.Sprintf("ns/%s %s/%s node/%s", event.InvolvedObject.Namespace, strings.ToLower(event.InvolvedObject.Kind), event.InvolvedObject.Name, event.Source.Host)
		}
		return fmt.Sprintf("ns/%s %s/%s", event.InvolvedObject.Namespace, strings.ToLower(event.InvolvedObject.Kind), event.InvolvedObject.Name)
	}
	if len(event.Source.Host) > 0 && event.InvolvedObject.Kind != "Node" {
		return fmt.Sprintf("%s/%s node/%s", strings.ToLower(event.InvolvedObject.Kind), event.InvolvedObject.Name, event.Source.Host)
	}
	return fmt.Sprintf("%s/%s", strings.ToLower(event.InvolvedObject.Kind), event.InvolvedObject.Name)
}

func locatePod(pod *corev1.Pod) string {
	return fmt.Sprintf("ns/%s pod/%s node/%s", pod.Namespace, pod.Name, pod.Spec.NodeName)
}

func locatePodContainer(pod *corev1.Pod, containerName string) string {
	return fmt.Sprintf("ns/%s pod/%s node/%s container/%s", pod.Namespace, pod.Name, pod.Spec.NodeName, containerName)
}

func filterToSystemNamespaces(obj runtime.Object) bool {
	m, ok := obj.(metav1.Object)
	if !ok {
		return true
	}
	ns := m.GetNamespace()
	if len(ns) == 0 {
		return true
	}
	return strings.HasPrefix(ns, "kube-") || strings.HasPrefix(ns, "openshift-") || ns == "default"
}

type errorRecordingListWatcher struct {
	lw cache.ListerWatcher

	recorder Recorder

	lock          sync.Mutex
	receivedError bool
}

func NewErrorRecordingListWatcher(recorder Recorder, lw cache.ListerWatcher) cache.ListerWatcher {
	return &errorRecordingListWatcher{
		lw:       lw,
		recorder: recorder,
	}
}

func (w *errorRecordingListWatcher) List(options metav1.ListOptions) (runtime.Object, error) {
	obj, err := w.lw.List(options)
	w.handle(err)
	return obj, err
}

func (w *errorRecordingListWatcher) Watch(options metav1.ListOptions) (watch.Interface, error) {
	obj, err := w.lw.Watch(options)
	w.handle(err)
	return obj, err
}

func (w *errorRecordingListWatcher) handle(err error) {
	w.lock.Lock()
	defer w.lock.Unlock()
	if err != nil {
		if !w.receivedError {
			w.recorder.Record(monitorapi.Condition{
				Level:   monitorapi.Error,
				Locator: "kube-apiserver",
				Message: fmt.Sprintf("failed contacting the API: %v", err),
			})
		}
		w.receivedError = true
	} else {
		w.receivedError = false
	}
}

// newKubeAPIEndpointsAwareRoundTripper creates a new HTTP round tripper for changing the destination host for each request to NodeIPs.
// The list of available servers is taken from the provided lister.
//
// See RoundTrip for more details.
func newKubeAPIEndpointsAwareRoundTripper(endpointsInformer informersv1.EndpointsInformer) func(http.RoundTripper) http.RoundTripper {
	return func(rt http.RoundTripper) http.RoundTripper {
		return &kubeAPIEndpointsAwareRT{delegate: rt, endpointsLister: endpointsInformer.Lister(), hasEndpointInformerSynced: endpointsInformer.Informer().HasSynced}
	}
}

type kubeAPIEndpointsAwareRT struct {
	delegate http.RoundTripper

	hasEndpointInformerSynced func() bool
	endpointsLister           listersv1.EndpointsLister

	index     int
	endpoints []string

	lock sync.Mutex
}

// RoundTrip sends the given request to a new Kube API Server on each invocation in a round robin fashion.
//
// It reads the current list of available servers from the lister.
// This seems to be okay since rolling a new kas instance is a slow process and might take many minutes.
// There is a huge window between starting the termination process and closing a socket.
// During that time we expect the server to process the requests without any disruption.
// In practice that window will be very short because the server removes the endpoint from the service upon receiving the SIGTERM.
func (rt *kubeAPIEndpointsAwareRT) RoundTrip(r *http.Request) (*http.Response, error) {
	if !rt.hasEndpointInformerSynced() {
		return nil, errors.New("unable to get the list of kubernetes endpoints since the cache hasn't been synced yet")
	}

	nextEndpoint, err := rt.nextEndpointLocked()
	if err != nil {
		return nil, err
	}
	r.Host = nextEndpoint
	r.URL.Host = nextEndpoint

	return rt.delegate.RoundTrip(r)
}

func (rt *kubeAPIEndpointsAwareRT) nextEndpointLocked() (string, error) {
	rt.lock.Lock()
	defer rt.lock.Unlock()

	eps, err := rt.endpointsLister.Endpoints(metav1.NamespaceDefault).Get("kubernetes")
	if err != nil {
		return "", err
	}
	kubeAPIEndpoints := []string{}
	for _, s := range eps.Subsets {
		var port int32
		for _, p := range s.Ports {
			if p.Name == "https" {
				port = p.Port
				break
			}
		}
		if port == 0 {
			continue
		}
		for _, ep := range s.Addresses {
			kubeAPIEndpoints = append(kubeAPIEndpoints, fmt.Sprintf("%s:%d", ep.IP, port))
		}
		break
	}
	if len(kubeAPIEndpoints) == 0 {
		return "", fmt.Errorf("%s/%s doesn't contain any valid endpoints", eps.Namespace, eps.Name)
	}

	// pick up a next server in a round robin fashion (approximation - when the list of endpoints changes)
	rt.endpoints = kubeAPIEndpoints
	rt.index = (rt.index + 1) % len(rt.endpoints)
	return rt.endpoints[rt.index], nil
}
