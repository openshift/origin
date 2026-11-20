package legacykubeapiservermonitortests

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// staticPodFailureRegex trying to pull out information from messages like
// `static pod lifecycle failure - static pod: "etcd" in namespace: "openshift-etcd" for revision: 6 on node: "ovirt10-gh8t5-master-2" didn't show up, waited: 2m30s`
var staticPodFailureRegex = regexp.MustCompile(
	`static pod lifecycle failure - static pod: "([^"]+)" in namespace: "([^"]+)" for revision: (\d+) on node: "([^"]+)" didn't show up, waited: .*`)

var staticPodReadyRegex = regexp.MustCompile(`^Updated node "([^"]+)" from revision [0-9]+ to ([0-9]+) because static pod is ready$`)

type staticPodFailure struct {
	operatorNamespace string
	podName           string
	podNamespace      string
	lastObserved      time.Time
	node              string
	revision          int64
	failureMessage    string
}

// staticPodFailureFromMessage extracts the pod identity and revision needed to match recovery evidence.
func staticPodFailureFromMessage(message string) (*staticPodFailure, error) {
	matches := staticPodFailureRegex.FindStringSubmatch(message)
	if len(matches) != 5 {
		return nil, fmt.Errorf("wrong number of matches: %v from: %q", matches, message)
	}
	revision, err := strconv.ParseInt(matches[3], 10, 64)
	if err != nil {
		return nil, err
	}

	return &staticPodFailure{
		podName:        matches[1],
		podNamespace:   matches[2],
		node:           matches[4],
		revision:       revision,
		failureMessage: message,
	}, nil
}

// testStaticPodLifecycleFailure reports failures without readiness evidence after their latest occurrence.
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
			staticPodFailure.lastObserved = staticPodEventTime(event)
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
		recovered, err := staticPodFailureRecovered(ctx, kubeClient, staticPodFailure, events.Items)
		if err != nil {
			failures = append(failures, err.Error())
		}
		if !recovered {
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

// staticPodEventTime includes aggregated repeats so readiness between occurrences
// cannot forgive a later failure.
func staticPodEventTime(event eventsv1.Event) time.Time {
	lastObserved := event.EventTime.Time
	if event.Series != nil && event.Series.LastObservedTime.After(lastObserved) {
		lastObserved = event.Series.LastObservedTime.Time
	}
	if event.DeprecatedLastTimestamp.After(lastObserved) {
		lastObserved = event.DeprecatedLastTimestamp.Time
	}
	return lastObserved
}

// staticPodFailureRecovered requires fresh readiness for the failed node and revision.
// Mirror-pod readiness handles same-revision restorations without revision-change events.
func staticPodFailureRecovered(ctx context.Context, kubeClient kubernetes.Interface, failure staticPodFailure, events []eventsv1.Event) (bool, error) {
	if failure.lastObserved.IsZero() {
		return false, nil
	}
	for _, event := range events {
		if event.Reason != "NodeCurrentRevisionChanged" || !staticPodEventTime(event).After(failure.lastObserved) {
			continue
		}
		matches := staticPodReadyRegex.FindStringSubmatch(event.Note)
		if len(matches) != 3 || matches[1] != failure.node {
			continue
		}
		revision, err := strconv.ParseInt(matches[2], 10, 64)
		if err == nil && revision >= failure.revision {
			return true, nil
		}
	}

	pod, err := kubeClient.CoreV1().Pods(failure.podNamespace).Get(ctx, failure.podName+"-"+failure.node, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, errors.New("checking static pod recovery: mirror pod lookup failed")
	}
	revision, err := strconv.ParseInt(pod.Labels["revision"], 10, 64)
	if err != nil || revision < failure.revision || pod.Spec.NodeName != failure.node ||
		len(pod.Annotations["kubernetes.io/config.mirror"]) == 0 || pod.DeletionTimestamp != nil || pod.Status.Phase != corev1.PodRunning {
		return false, nil
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue && condition.LastTransitionTime.After(failure.lastObserved) {
			return true, nil
		}
	}
	return false, nil
}
