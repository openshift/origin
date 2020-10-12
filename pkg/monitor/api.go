package monitor

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"k8s.io/client-go/transport"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	configclientset "github.com/openshift/client-go/config/clientset/versioned"
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
	if err := StartOpenShiftAPIMonitoringWithNewConnections(ctx, m, clusterConfig, 5*time.Second); err != nil {
		return nil, err
	}
	if err := StartOAuthAPIMonitoringWithNewConnections(ctx, m, clusterConfig, 5*time.Second); err != nil {
		return nil, err
	}
	if err := StartKubeAPIMonitoringWithConnectionReuse(ctx, m, clusterConfig, 5*time.Second); err != nil {
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
	startClusterOperatorMonitoring(ctx, m, configClient)

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

	httpClient := http.Client{
		Transport: roundTripper,
	}

	m.AddSampler(
		StartSampling(ctx, m, time.Second, func(previous bool) (condition *Condition, next bool) {
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
				condition = &Condition{
					Level:   Info,
					Locator: resourceLocator,
					Message: fmt.Sprintf("%s started responding to GET requests", resourceLocator),
				}
			case err != nil && previous:
				condition = &Condition{
					Level:   Error,
					Locator: resourceLocator,
					Message: fmt.Sprintf("%s started failing: %v", resourceLocator, err),
				}
			}
			return condition, err == nil
		}).ConditionWhenFailing(&Condition{
			Level:   Error,
			Locator: resourceLocator,
			Message: fmt.Sprintf("%s is not responding to GET requests", resourceLocator),
		}),
	)
	return nil
}

func StartKubeAPIMonitoringWithNewConnections(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration) error {
	// default gets auto-created, so this should always exist
	return StartServerMonitoringWithNewConnections(ctx, m, clusterConfig, timeout, "kube-apiserver-new-connection", "/api/v1/namespaces/default")
}

func StartOpenShiftAPIMonitoringWithNewConnections(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration) error {
	// this request should never 404, but should be empty/small
	return StartServerMonitoringWithNewConnections(ctx, m, clusterConfig, timeout, "openshift-apiserver-new-connection", "/apis/image.openshift.io/v1/namespaces/default/imagestreams")
}

func StartOAuthAPIMonitoringWithNewConnections(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration) error {
	// this should be relatively small and should not ever 404
	return StartServerMonitoringWithNewConnections(ctx, m, clusterConfig, timeout, "oauth-apiserver-new-connection", "/apis/oauth.openshift.io/v1/oauthclients")
}

func StartKubeAPIMonitoringWithConnectionReuse(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration) error {
	// default gets auto-created, so this should always exist
	return StartServerMonitoringWithConnectionReuse(ctx, m, clusterConfig, timeout, "kube-apiserver-reuse-connection", "/api/v1/namespaces/default")
}

func StartOpenShiftAPIMonitoringWithConnectionReuse(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration) error {
	// this request should never 404, but should be empty/small
	return StartServerMonitoringWithConnectionReuse(ctx, m, clusterConfig, timeout, "openshift-apiserver-reuse-connection", "/apis/image.openshift.io/v1/namespaces/default/imagestreams")
}

func StartOAuthAPIMonitoringWithConnectionReuse(ctx context.Context, m *Monitor, clusterConfig *rest.Config, timeout time.Duration) error {
	// this should be relatively small and should not ever 404
	return StartServerMonitoringWithConnectionReuse(ctx, m, clusterConfig, timeout, "oauth-apiserver-reuse-connection", "/apis/oauth.openshift.io/v1/oauthclients")
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

func locateNode(node *corev1.Node) string {
	return fmt.Sprintf("node/%s", node.Name)
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
			w.recorder.Record(Condition{
				Level:   Error,
				Locator: "kube-apiserver",
				Message: fmt.Sprintf("failed contacting the API: %v", err),
			})
		}
		w.receivedError = true
	} else {
		w.receivedError = false
	}
}
