package synthetictests

import (
	"context"
	"fmt"
	"strings"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func testStaticPodLifecycleFailure(events monitorapi.Intervals, kubeClientConfig *rest.Config, testSuite string) []*junitapi.JUnitTestCase {
	ctx := context.TODO()
	const testName = `[sig-node] static pods should start after being created`
	failures := []string{}

	kubeClient, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return []*junitapi.JUnitTestCase{
			{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: err.Error(),
				},
				SystemOut: err.Error(),
			},
		}
	}

	staticPodNamespaces := []string{
		"openshift-etcd-operator",
		"openshift-kube-apiserver-operator",
		"openshift-kube-controller-manager-operator",
		"openshift-kube-scheduler-operator",
	}
	for _, ns := range staticPodNamespaces {
		// we need to get all the events from the cluster, so we cannot use the monitor events.
		events, err := kubeClient.EventsV1().Events(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}

		for _, event := range events.Items {
			if !strings.Contains(event.Note, "static pod lifecycle failure") {
				continue
			}
			failures = append(failures, fmt.Sprintf("%#v", event))
		}
	}

	if len(failures) > 0 {
		return []*junitapi.JUnitTestCase{
			{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: strings.Join(failures, "\n"),
				},
				SystemOut: strings.Join(failures, "\n"),
			},
		}
	}

	return []*junitapi.JUnitTestCase{
		{
			Name: testName,
		},
	}
}
