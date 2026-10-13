package disruptionlibrary

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

func CollectIntervalsForPods(ctx context.Context, kubeClient kubernetes.Interface, sig string, namespace string, labelSelector labels.Selector) (monitorapi.Intervals, []*junitapi.JUnitTestCase, []error) {
	pollerPods, err := kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
	if err != nil {
		return nil, nil, []error{err}
	}

	retIntervals := monitorapi.Intervals{}
	junits := []*junitapi.JUnitTestCase{}
	errs := []error{}
	buf := &bytes.Buffer{}
	podsWithoutIntervals := []string{}
	for _, pollerPod := range pollerPods.Items {
		fmt.Fprintf(buf, "\n\nLogs for -n %v pod/%v\n", pollerPod.Namespace, pollerPod.Name)
		req := kubeClient.CoreV1().Pods(namespace).GetLogs(pollerPod.Name, &corev1.PodLogOptions{})
		if err != nil {
			errs = append(errs, err)
			continue
		}
		logStream, err := req.Stream(ctx)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		foundInterval := false
		scanner := bufio.NewScanner(logStream)
		for scanner.Scan() {
			line := scanner.Bytes()
			buf.Write(line)
			buf.Write([]byte("\n"))
			if len(line) == 0 {
				continue
			}

			// not all lines are json, ignore errors.
			if currInterval, err := monitorserialization.IntervalFromJSON(line); err == nil {
				retIntervals = append(retIntervals, *currInterval)
				foundInterval = true
			}
		}
		if !foundInterval {
			podsWithoutIntervals = append(podsWithoutIntervals, pollerPod.Name)
		}
	}

	failures := []string{}
	if len(podsWithoutIntervals) > 0 {
		failures = append(failures, fmt.Sprintf("%d pods lacked sampler output: [%v]", len(podsWithoutIntervals), strings.Join(podsWithoutIntervals, ", ")))
	}
	if len(pollerPods.Items) == 0 {
		failures = append(failures, fmt.Sprintf("no pods found for poller %v", labelSelector))
	}

	logJunit := &junitapi.JUnitTestCase{
		Name:      fmt.Sprintf("[%s] can collect %v poller pod logs", sig, labelSelector),
		SystemOut: string(buf.Bytes()),
	}
	if len(failures) > 0 {
		logJunit.FailureOutput = &junitapi.FailureOutput{
			Output: strings.Join(failures, "\n"),
		}
	}
	junits = append(junits, logJunit)

	return retIntervals, junits, errs
}
