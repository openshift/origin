package monitor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/test/utils"
)

// pods whose metrics show a larger ratio of requests per
// second than maxQPSAllowed are considered "unhealthy".
const (
	maxQPSAllowed = 1.5
)

var (
	// TODO: these exceptions should not exist. Update operators to have a better request-rate per second
	perComponentNamespaceMaxQPSAllowed = map[string]float64{
		"openshift-apiserver-operator":                            7.2,
		"openshift-kube-apiserver-operator":                       7.2,
		"openshift-kube-controller-manager-operator":              2.0,
		"openshift-cluster-kube-scheduler-operator":               1.8,
		"openshift-cluster-openshift-controller-manager-operator": 1.7,
		"openshift-kube-scheduler-operator":                       1.7,
	}
)

func startOperatorMonitoring(ctx context.Context, config *rest.Config, m *Monitor, client kubernetes.Interface) {
	podURLGetter := &portForwardURLGetter{
		Protocol:   "https",
		Host:       "localhost",
		RemotePort: "8443",
		LocalPort:  "37587",
	}

	podInformer := cache.NewSharedIndexInformer(
		NewErrorRecordingListWatcher(m, &cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				items, err := client.CoreV1().Pods("").List(options)
				if err == nil {
					last := 0
					for i := range items.Items {
						item := &items.Items[i]
						if !filterToOperatorPods(item) {
							continue
						}
						if i != last {
							items.Items[last] = *item
							last++
						}
					}
					items.Items = items.Items[:last]
				}
				return items, err
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				w, err := client.CoreV1().Pods("").Watch(options)
				if err == nil {
					w = watch.Filter(w, func(in watch.Event) (watch.Event, bool) {
						return in, filterToOperatorPods(in.Object)
					})
				}
				return w, err
			},
		}),
		&corev1.Pod{},
		10*time.Minute,
		nil,
	)

	m.AddSampler(func(now time.Time) []*Condition {
		var conditions []*Condition
		for _, obj := range podInformer.GetStore().List() {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				continue
			}

			podReady, err := utils.PodRunningReady(pod)
			if !podReady || err != nil {
				conditions = append(conditions, &Condition{
					Level:   Warning,
					Locator: locatePod(pod),
					Message: "skipped, pod is not Running",
				})
				continue
			}
			if len(pod.Spec.Containers) == 0 {
				conditions = append(conditions, &Condition{
					Level:   Warning,
					Locator: locatePod(pod),
					Message: "skipped, no containers found",
				})
				continue
			}

			metrics, err := getPodMetrics(config, pod, podURLGetter)
			if err != nil {
				switch err.(type) {
				case *url.Error:
					if strings.Contains(err.Error(), "EOF") {
						conditions = append(conditions, &Condition{
							Level:   Warning,
							Locator: locatePod(pod),
							Message: fmt.Sprintf("/metrics endpoint not available"),
						})
					}
				default:
					conditions = append(conditions, &Condition{
						Level:   Error,
						Locator: locatePod(pod),
						Message: fmt.Sprintf("error retrieving pod metrics: %v", err),
					})
				}
				continue
			}

			metricGroup, ok := metrics["rest_client_requests_total"]
			if !ok {
				conditions = append(conditions, &Condition{
					Level:   Error,
					Locator: locatePod(pod),
					Message: fmt.Sprintf("error: failed to find counter: %q", "rest_client_requests_total"),
				})
				continue
			}

			procStartTime, ok := metrics["process_start_time_seconds"]
			if !ok || len(procStartTime.Metric) == 0 {
				conditions = append(conditions, &Condition{
					Level:   Error,
					Locator: locatePod(pod),
					Message: fmt.Sprintf("error: failed to find metric: %q", "process_start_time_seconds"),
				})
				continue
			}
			procStartTimeSeconds := procStartTime.Metric[0].GetGauge().GetValue()
			totalProcTimeSeconds := time.Now().Unix() - int64(procStartTimeSeconds)

			totalRequestCount := float64(0)
			for _, metric := range metricGroup.Metric {
				totalRequestCount += metric.Counter.GetValue()
			}

			qpsLimit := maxQPSAllowed
			if customLimit, ok := perComponentNamespaceMaxQPSAllowed[pod.Namespace]; ok {
				qpsLimit = customLimit
			}
			qps := totalRequestCount / float64(totalProcTimeSeconds)

			if qps > qpsLimit {
				conditions = append(conditions, &Condition{
					Level:   Error,
					Locator: locatePod(pod),
					Message: fmt.Sprintf("%v requests over a span of %v seconds", totalRequestCount, totalProcTimeSeconds),
				})
				continue
			}
		}
		return conditions
	})

	podInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {},
			DeleteFunc: func(obj interface{}) {
				pod, ok := obj.(*corev1.Pod)
				if !ok {
					return
				}
				m.Record(Condition{
					Level:   Warning,
					Locator: locatePod(pod),
					Message: "deleted",
				})
			},
			UpdateFunc: func(old, obj interface{}) {
				pod, ok := obj.(*corev1.Pod)
				if !ok {
					return
				}
				oldPod, ok := old.(*corev1.Pod)
				if !ok {
					return
				}
				if pod.UID != oldPod.UID {
					return
				}
			},
		},
	)

	go podInformer.Run(ctx.Done())
}

func filterToOperatorPods(obj runtime.Object) bool {
	m, ok := obj.(metav1.Object)
	if !ok {
		return false
	}

	if !strings.HasPrefix(m.GetNamespace(), "openshift-") || !strings.HasSuffix(m.GetNamespace(), "-operator") {
		return false
	}
	if !strings.Contains(m.GetName(), "-operator-") {
		return false
	}
	return true
}

func getPodMetrics(adminConfig *restclient.Config, pod *corev1.Pod, podURLGetter *portForwardURLGetter) (map[string]*dto.MetricFamily, error) {
	result, err := podURLGetter.Get("/metrics", pod, adminConfig)
	if err != nil {
		return nil, err
	}

	return parseRawMetrics(result)
}

func parseRawMetrics(rawMetrics string) (map[string]*dto.MetricFamily, error) {
	p := expfmt.TextParser{}
	return p.TextToMetricFamilies(bytes.NewBufferString(rawMetrics))
}

type defaultPortForwarder struct {
	restConfig *rest.Config

	StopChannel  chan struct{}
	ReadyChannel chan struct{}
}

func newDefaultPortForwarder(adminConfig *rest.Config) *defaultPortForwarder {
	return &defaultPortForwarder{
		restConfig:   adminConfig,
		StopChannel:  make(chan struct{}, 1),
		ReadyChannel: make(chan struct{}, 1),
	}
}

func (f *defaultPortForwarder) forwardPortsAndExecute(pod *corev1.Pod, ports []string, toExecute func()) error {
	if len(ports) < 1 {
		return fmt.Errorf("at least 1 PORT is required for port-forward")
	}

	restClient, err := rest.RESTClientFor(setRESTConfigDefaults(*f.restConfig))
	if err != nil {
		return err
	}

	if pod.Status.Phase != corev1.PodRunning {
		return fmt.Errorf("unable to forward port because pod is not running. Current status=%v", pod.Status.Phase)
	}

	stdout := bytes.NewBuffer(nil)
	req := restClient.Post().
		Resource("pods").
		Namespace(pod.Namespace).
		Name(pod.Name).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(f.restConfig)
	if err != nil {
		return err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())
	fw, err := portforward.New(dialer, ports, f.StopChannel, f.ReadyChannel, stdout, stdout)
	if err != nil {
		return err
	}

	go func() {
		if f.StopChannel != nil {
			defer close(f.StopChannel)
		}

		<-f.ReadyChannel
		toExecute()
	}()

	return fw.ForwardPorts()
}

func setRESTConfigDefaults(config rest.Config) *rest.Config {
	if config.GroupVersion == nil {
		config.GroupVersion = &schema.GroupVersion{Group: "", Version: "v1"}
	}
	if config.NegotiatedSerializer == nil {
		config.NegotiatedSerializer = scheme.Codecs
	}
	if len(config.UserAgent) == 0 {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	config.APIPath = "/api"
	return &config
}

func newInsecureRESTClientForHost(host string) (rest.Interface, error) {
	insecure := true

	configFlags := &genericclioptions.ConfigFlags{}
	configFlags.Insecure = &insecure
	configFlags.APIServer = &host

	newConfig, err := configFlags.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	return rest.RESTClientFor(setRESTConfigDefaults(*newConfig))
}

type portForwardURLGetter struct {
	Protocol   string
	Host       string
	RemotePort string
	LocalPort  string
}

// Get receives a url path (i.e. /metrics), a pod, and a rest config, and forwards a set remote port on the pod
// to a specified local port. It then executes a GET request using an insecure REST client against the given urlPath.
func (c *portForwardURLGetter) Get(urlPath string, pod *corev1.Pod, config *rest.Config) (string, error) {
	var result string
	var lastErr error
	forwarder := newDefaultPortForwarder(config)

	if err := forwarder.forwardPortsAndExecute(pod, []string{c.LocalPort + ":" + c.RemotePort}, func() {
		restClient, err := newInsecureRESTClientForHost(fmt.Sprintf("https://localhost:%s/", c.LocalPort))
		if err != nil {
			lastErr = err
			return
		}

		ioCloser, err := restClient.Get().RequestURI(urlPath).Stream()
		if err != nil {
			lastErr = err
			return
		}
		defer ioCloser.Close()

		data := bytes.NewBuffer(nil)
		_, lastErr = io.Copy(data, ioCloser)
		result = data.String()
	}); err != nil {
		return "", err
	}
	return result, lastErr
}
