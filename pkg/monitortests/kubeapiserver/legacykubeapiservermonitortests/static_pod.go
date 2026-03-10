package legacykubeapiservermonitortests

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// staticPodFailureRegex trying to pull out information from messages like
// `static pod lifecycle failure - static pod: "etcd" in namespace: "openshift-etcd" for revision: 6 on node: "ovirt10-gh8t5-master-2" didn't show up, waited: 2m30s`
var staticPodFailureRegex = regexp.MustCompile(
	`static pod lifecycle failure - static pod: ".*" in namespace: ".*" for revision: (\d+) on node: "(.*)" didn't show up, waited: .*`)

type staticPodFailure struct {
	operatorNamespace string
	node              string
	revision          int64
	failureMessage    string
}

func staticPodFailureFromMessage(message string) (*staticPodFailure, error) {
	matches := staticPodFailureRegex.FindStringSubmatch(message)
	if len(matches) != 3 {
		return nil, fmt.Errorf("wrong number of matches: %v from: %q", matches, message)
	}
	revision, err := strconv.ParseInt(matches[1], 0, 64)
	if err != nil {
		return nil, err
	}

	return &staticPodFailure{
		node:           matches[2],
		revision:       revision,
		failureMessage: message,
	}, nil
}

func testStaticPodLifecycleFailure(events monitorapi.Intervals, kubeClientConfig *rest.Config) []*junitapi.JUnitTestCase {
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
	staticPodFailures := []staticPodFailure{}
	for _, ns := range staticPodNamespaces {
		// we need to get all the events from the cluster, so we cannot use the monitor events.
		events, err := kubeClient.EventsV1().Events(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}

		for _, event := range events.Items {
			if event.Reason == "OperatorStatusChanged" { // skip the clusteroperator status change event.
				continue
			}
			if !strings.Contains(event.Note, "static pod lifecycle failure") {
				continue
			}

			staticPodFailure, err := staticPodFailureFromMessage(event.Note)
			if err != nil {
				failures = append(failures, fmt.Sprintf("%v", err))
				continue
			}
			staticPodFailure.operatorNamespace = ns
			staticPodFailures = append(staticPodFailures, *staticPodFailure)
		}
	}

	// now check each failure to see if we eventually reached the level.  If we eventually reached the level
	// then don't flag it
	for _, staticPodFailure := range staticPodFailures {
		events, err := kubeClient.EventsV1().Events(staticPodFailure.operatorNamespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		foundEventForProperRevision := false
		for _, event := range events.Items {
			isRevisionUpdate := event.Reason == "NodeCurrentRevisionChanged"
			isForNode := strings.Contains(event.Note, staticPodFailure.node)
			matches := regexp.MustCompile("to ([0-9]+) because static pod is ready").FindStringSubmatch(event.Note)
			if len(matches) == 2 {
				reachedRevision, _ := strconv.ParseInt(matches[1], 0, 64)
				if isRevisionUpdate && isForNode && reachedRevision >= staticPodFailure.revision {
					// If we reach the level eventually, don't fail the test. We might choose to add an "it's slow" test, but
					// it hasn't failed. It is possible to go directly to a later revision such as in a RT environment where
					// revisions can happen rapidly. In this case, it is okay if a revision never appears and a later revision
					// is located.
					foundEventForProperRevision = true
				}
			}
		}

		if !foundEventForProperRevision {
			failures = append(failures, staticPodFailure.failureMessage)
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
