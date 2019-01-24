package operators

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"

	"k8s.io/api/core/v1"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/kubernetes/pkg/client/conditions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/utils"

	exutil "github.com/openshift/origin/test/extended/util"
)

type podInfo struct {
	name      string
	qps       float64
	status    string
	namespace string
	result    string
	failed    bool
	skipped   bool
	pod       *v1.Pod
}

func locatePrometheus(oc *exutil.CLI) bool {
	_, err := oc.AdminKubeClient().Core().Services("openshift-monitoring").Get("prometheus-k8s", metav1.GetOptions{})
	if kapierrs.IsNotFound(err) {
		return false
	}
	bearerToken := ""

	waitForServiceAccountInNamespace(oc.AdminKubeClient(), "openshift-monitoring", "prometheus-k8s", 2*time.Minute)
	for i := 0; i < 30; i++ {
		secrets, err := oc.AdminKubeClient().Core().Secrets("openshift-monitoring").List(metav1.ListOptions{})
		if err != nil {
			panic(err)
		}

		for _, secret := range secrets.Items {
			if secret.Type != v1.SecretTypeServiceAccountToken {
				continue
			}
			if !strings.HasPrefix(secret.Name, "prometheus-") {
				continue
			}
			bearerToken = string(secret.Data[v1.ServiceAccountTokenKey])
			break
		}
		if len(bearerToken) == 0 {
			e2e.Logf("Waiting for prometheus service account secret to show up")
			time.Sleep(time.Second)
			continue
		}
	}
	if len(bearerToken) == 0 {
		return false
	}

	return true
}

func waitForServiceAccountInNamespace(c clientset.Interface, ns, serviceAccountName string, timeout time.Duration) error {
	w, err := c.Core().ServiceAccounts(ns).Watch(metav1.SingleObject(metav1.ObjectMeta{Name: serviceAccountName}))
	if err != nil {
		return err
	}
	_, err = watch.Until(timeout, w, conditions.ServiceAccountHasSecrets)
	return err
}

func getPodInfoForNamespace(adminClient kubernetes.Interface, adminConfig *restclient.Config, podURLGetter *portForwardURLGetter, namespace string) ([]*podInfo, error) {
	pods, err := adminClient.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	podInfos := []*podInfo{}
	for _, pod := range pods.Items {
		// skip non-operator pods
		if !strings.Contains(pod.Name, "-operator-") {
			continue
		}

		info := &podInfo{
			name:      pod.Name,
			namespace: pod.Namespace,
			status:    string(pod.Status.Phase),
			pod:       &pod,
		}

		podReady, err := utils.PodRunningReady(&pod)
		if !podReady || err != nil {
			result := "skipped, pod is not Running"
			if err != nil {
				result = fmt.Sprintf("%s: %v", result, err)
			}

			info.result = result
			info.skipped = true
			podInfos = append(podInfos, info)
			continue
		}

		if len(pod.Spec.Containers) == 0 {
			info.result = "skipped, no containers found"
			info.skipped = true
			podInfos = append(podInfos, info)
			continue
		}

		metrics, err := getPodMetrics(adminConfig, &pod, podURLGetter)
		if err != nil {
			info.result = fmt.Sprintf("error: %s", err)
			info.failed = true

			// ignore errors from pods with no /metrics endpoint available
			switch err.(type) {
			case *url.Error:
				if strings.Contains(err.Error(), "EOF") {
					info.skipped = true
					info.failed = false
					info.result = fmt.Sprintf("/metrics endpoint not available")
				}
			}

			podInfos = append(podInfos, info)
			continue
		}

		metricGroup, ok := metrics["rest_client_requests_total"]
		if !ok {
			info.result = fmt.Sprintf("error: failed to find counter: %q", "rest_client_requests_total")
			info.failed = true
			podInfos = append(podInfos, info)
			continue
		}

		procStartTime, ok := metrics["process_start_time_seconds"]
		if !ok || len(procStartTime.Metric) == 0 {
			info.result = fmt.Sprintf("error: failed to find metric: %q", "process_start_time_seconds")
			info.failed = true
			podInfos = append(podInfos, info)
			continue
		}
		procStartTimeSeconds := procStartTime.Metric[0].GetGauge().GetValue()
		totalProcTimeSeconds := time.Now().Unix() - int64(procStartTimeSeconds)

		totalRequestCount := float64(0)
		for _, metric := range metricGroup.Metric {
			totalRequestCount += metric.Counter.GetValue()
		}

		comment := "within QPS bounds"
		qps := totalRequestCount / float64(totalProcTimeSeconds)
		if qps > maxQPSAllowed {
			comment = "exceeds QPS bounds"
		}
		info.status = fmt.Sprintf("%s (%s)", info.status, comment)
		info.qps = qps
		info.result = fmt.Sprintf("%v requests over a span of %v seconds", totalRequestCount, totalProcTimeSeconds)
		podInfos = append(podInfos, info)
	}

	return podInfos, nil
}

func getPodMetrics(adminConfig *restclient.Config, pod *v1.Pod, podURLGetter *portForwardURLGetter) (map[string]*dto.MetricFamily, error) {
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

func (f *defaultPortForwarder) forwardPortsAndExecute(pod *v1.Pod, ports []string, toExecute func()) error {
	if len(ports) < 1 {
		return fmt.Errorf("at least 1 PORT is required for port-forward")
	}

	restClient, err := rest.RESTClientFor(setRESTConfigDefaults(*f.restConfig))
	if err != nil {
		return err
	}

	if pod.Status.Phase != v1.PodRunning {
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
func (c *portForwardURLGetter) Get(urlPath string, pod *v1.Pod, config *rest.Config) (string, error) {
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
