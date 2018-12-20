package operators

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"strings"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	clientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubernetes/pkg/kubectl/util/term"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

// pods whose metrics show a larger ratio of requests per
// second than maxQPSAllowed are considered "unhealthy".
const maxQPSAllowed = 1.5

type podInfo struct {
	name      string
	qps       float64
	status    string
	namespace string
	result    string
	failed    bool
}

func calculatePodMetrics(oc *exutil.CLI) error {
	namespaces, err := oc.AdminKubeClient().CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, ns := range namespaces.Items {
		// skip namespaces which do not meet "operator namespace" criteria
		if !strings.HasPrefix(ns.Name, "openshift-") {
			continue
		}
		if !strings.HasSuffix(ns.Name, "-operator") {
			continue
		}

		infos, err := getPodInfoForNamespace(oc, ns.Name)
		if err != nil {
			return err
		}

		for _, info := range infos {
			if info.failed {
				e2e.Logf("Failed to fetch operator pod metrics for pod %q: %s", info.name, info.result)
				continue
			}

			if info.qps > maxQPSAllowed {
				return fmt.Errorf("operator pod %q in namespace %q is making %v requests per second. Maximum allowed is %v per second", info.name, info.namespace, info.qps, maxQPSAllowed)
			}
		}
	}
	return nil
}

func getPodInfoForNamespace(oc *exutil.CLI, namespace string) ([]*podInfo, error) {
	pods, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(metav1.ListOptions{})
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

		if len(pod.Spec.Containers) == 0 {
			info.result = "skipped, no containers found"
			info.failed = true
			podInfos = append(podInfos, info)
			continue
		}

		metrics, err := getPodMetrics(oc, &pod)
		if err != nil {
			info.result = fmt.Sprintf("error: %s", err)
			info.failed = true
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
			info.result = fmt.Sprintf("error: failed to obtain total cpu time for operator process")
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

func getPodMetrics(oc *exutil.CLI, pod *v1.Pod) (map[string]*dto.MetricFamily, error) {
	if len(oc.AdminConfig().BearerToken) == 0 {
		return nil, fmt.Errorf("no Bearer Token found in provided admin config")
	}

	command := []string{"/bin/curl", "-H", fmt.Sprintf("%s", "Authorization: Bearer "+oc.AdminConfig().BearerToken), "-k", "https://localhost:8443/metrics"}
	result, err := execCommandInPod(oc, pod, command)
	if err != nil {
		return nil, err
	}

	return parseRawMetrics(result)
}

func parseRawMetrics(rawMetrics string) (map[string]*dto.MetricFamily, error) {
	p := expfmt.TextParser{}
	return p.TextToMetricFamilies(bytes.NewBufferString(rawMetrics))
}

func execCommandInPod(oc *exutil.CLI, pod *v1.Pod, command []string) (string, error) {
	cmdOutput := bytes.NewBuffer(nil)

	if len(pod.Spec.Containers) == 0 {
		return "", fmt.Errorf("no containers found")
	}

	t := term.TTY{
		Out: os.Stdout,
	}
	if err := t.Safe(func() error {
		restClient, err := clientv1.NewForConfig(oc.AdminConfig())
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
		return executor.Execute("POST", req.URL(), oc.AdminConfig(), nil, cmdOutput, ioutil.Discard, false, sizeQueue)
	}); err != nil {
		return "", err
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
