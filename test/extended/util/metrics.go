package util

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	clientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubernetes/pkg/kubectl/util/term"
	"k8s.io/kubernetes/test/utils"
)

// pods whose metrics show a larger ratio of requests per
// second than maxQPSAllowed are considered "unhealthy".
const maxQPSAllowed = 1.5

var (
	// TODO: these exceptions should not exist. Update operators to have a better request-rate per second
	perComponentNamespaceMaxQPSAllowed = map[string]float64{
		"openshift-apiserver-operator":                            3.0,
		"openshift-kube-apiserver-operator":                       6.8,
		"openshift-kube-controller-manager-operator":              2.0,
		"openshift-cluster-kube-scheduler-operator":               1.8,
		"openshift-cluster-openshift-controller-manager-operator": 1.7,
	}
)

type podInfo struct {
	name      string
	qps       float64
	status    string
	namespace string
	result    string
	failed    bool
	skipped   bool
}

func calculatePodMetrics(adminClient kubernetes.Interface, adminConfig *restclient.Config) error {
	namespaces, err := adminClient.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	bearerToken, err := getBearerToken(adminClient, "openshift-apiserver")
	if err != nil {
		return err
	}

	failures := []error{}
	for _, ns := range namespaces.Items {
		// skip namespaces which do not meet "operator namespace" criteria
		if !strings.HasPrefix(ns.Name, "openshift-") || !strings.HasSuffix(ns.Name, "-operator") {
			continue
		}

		infos, err := getPodInfoForNamespace(adminClient, adminConfig, ns.Name, bearerToken)
		if err != nil {
			return err
		}

		for _, info := range infos {
			if info.failed {
				failures = append(failures, fmt.Errorf("Failed to fetch operator pod metrics for pod %q: %s", info.name, info.result))
				continue
			}
			if info.skipped {
				continue
			}

			qpsLimit := maxQPSAllowed
			if customLimit, ok := perComponentNamespaceMaxQPSAllowed[info.namespace]; ok {
				qpsLimit = customLimit
			}

			if info.qps > qpsLimit {
				failures = append(failures, fmt.Errorf("operator pod %q in namespace %q is making %v requests per second. Maximum allowed is %v requests per second", info.name, info.namespace, info.qps, maxQPSAllowed))
				continue
			}
		}
	}

	if len(failures) > 0 {
		return errors.NewAggregate(failures)
	}
	return nil
}

func getPodInfoForNamespace(adminClient kubernetes.Interface, adminConfig *restclient.Config, namespace, bearerToken string) ([]*podInfo, error) {
	pods, err := adminClient.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	podInfos := []*podInfo{}
	for _, pod := range pods.Items {
		info := &podInfo{
			name:      pod.Name,
			namespace: pod.Namespace,
			status:    string(pod.Status.Phase),
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

		metrics, err := getPodMetrics(adminConfig, &pod, bearerToken)
		if err != nil {
			// ignore errors from pods with no /metrics endpoint available
			if !strings.Contains(err.Error(), "Connection refused") {
				info.result = fmt.Sprintf("error: %s", err)
				info.failed = true
			} else {
				info.skipped = true
				info.result = fmt.Sprintf("/metrics endpoint not available")
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

func getBearerToken(adminClient kubernetes.Interface, namespace string) (string, error) {
	secrets, err := adminClient.CoreV1().Secrets(namespace).List(metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	bearerToken := ""
	for _, s := range secrets.Items {
		if !strings.HasPrefix(s.Name, namespace) {
			continue
		}
		if s.Type != v1.SecretTypeServiceAccountToken {
			continue
		}
		bearerToken = string(s.Data[v1.ServiceAccountTokenKey])
		break
	}
	if len(bearerToken) == 0 {
		return "", fmt.Errorf("no Bearer Token found for SA in namespace %q", namespace)
	}

	return bearerToken, nil
}

func getPodMetrics(adminConfig *restclient.Config, pod *v1.Pod, token string) (map[string]*dto.MetricFamily, error) {
	if len(token) == 0 {
		return nil, fmt.Errorf("no Bearer Token found in provided admin config")
	}

	command := []string{"/bin/curl", "-H", fmt.Sprintf("%s", "Authorization: Bearer "+token), "-k", "https://localhost:8443/metrics"}
	result, err := execCommandInPod(adminConfig, pod, command)
	if err != nil {
		return nil, err
	}

	return parseRawMetrics(result)
}

func parseRawMetrics(rawMetrics string) (map[string]*dto.MetricFamily, error) {
	p := expfmt.TextParser{}
	return p.TextToMetricFamilies(bytes.NewBufferString(rawMetrics))
}

func execCommandInPod(adminConfig *restclient.Config, pod *v1.Pod, command []string) (string, error) {
	cmdOutput := bytes.NewBuffer(nil)
	cmdErr := bytes.NewBuffer(nil)

	if len(pod.Spec.Containers) == 0 {
		return "", fmt.Errorf("no containers found")
	}

	t := term.TTY{
		Out: os.Stdout,
	}
	if err := t.Safe(func() error {
		restClient, err := clientv1.NewForConfig(adminConfig)
		if err != nil {
			return err
		}

		containerName := pod.Spec.Containers[0].Name
		req := restClient.RESTClient().Post().
			Resource("pods").
			Name(pod.GetName()).
			Namespace(pod.GetNamespace()).
			SubResource("exec").
			Param("container", containerName)
		req.VersionedParams(&v1.PodExecOptions{
			Container: containerName,
			Command:   command,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

		var sizeQueue remotecommand.TerminalSizeQueue
		executor := &remoteExecutor{}
		return executor.Execute("POST", req.URL(), adminConfig, nil, cmdOutput, cmdErr, false, sizeQueue)
	}); err != nil {
		return "", fmt.Errorf("%v: %v", cmdErr.String(), err)
	}

	return cmdOutput.String(), nil
}

type remoteExecutor struct{}

func (*remoteExecutor) Execute(method string, url *url.URL, config *restclient.Config, stdin io.Reader, stdout, stderr io.Writer, tty bool, terminalSizeQueue remotecommand.TerminalSizeQueue) error {
	exec, err := remotecommand.NewSPDYExecutor(config, method, url)
	if err != nil {
		return err
	}

	return exec.Stream(remotecommand.StreamOptions{
		Stdin:             stdin,
		Stdout:            stdout,
		Stderr:            stderr,
		Tty:               tty,
		TerminalSizeQueue: terminalSizeQueue,
	})
}
