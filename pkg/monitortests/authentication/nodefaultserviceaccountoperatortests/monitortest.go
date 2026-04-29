package nodefaultserviceaccountoperatortests

import (
	"context"
	"fmt"
	"strings"
	"time"

	configv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type noDefaultServiceAccountChecker struct {
	kubeClient kubernetes.Interface
	cfgClient  *configv1.ConfigV1Client
}

func NewAnalyzer() monitortestframework.MonitorTest {
	return &noDefaultServiceAccountChecker{}
}

// Cleanup implements monitortestframework.MonitorTest.
func (n *noDefaultServiceAccountChecker) Cleanup(ctx context.Context) error {
	return nil
}

func exceptionWithJira(prefix, jiraURL string) func(corev1.Pod) (string, bool) {
	return func(pod corev1.Pod) (string, bool) {
		podNameNSCombo := pod.Namespace + "/" + pod.Name
		if strings.HasPrefix(podNameNSCombo, prefix) {
			return jiraURL, true
		}
		return "", false
	}
}

// OpenShift components should not be using the default service account.
// Therefore, no new components should be added to this list.
// The following are current exceptions due to not being part of the core OpenShift
// payload or have open PRs to resolve.
var exceptions = []func(pod corev1.Pod) (string, bool){
	// This exception is being kept as there is still an open PR for this ticket.
	// TODO(ehearne-redhat): check back on this ticket to review progress.
	exceptionWithJira("openshift-multus/multus-", "https://issues.redhat.com/browse/OCPBUGS-65631"),
	exceptionWithJira("openshift-route-monitor-operator/blackbox-exporter-", "https://redhat.atlassian.net/browse/SREP-4724"),
	exceptionWithJira("openshift-security/", "https://redhat.atlassian.net/browse/SREP-4725"),
	// This one checks if it is a debug pod or not.
	// debug pod does not run by default on an OpenShift cluster.
	// debug pod has showed up in some e2e tests. In order to
	// stop these tests from failing we include it here.
	func(pod corev1.Pod) (string, bool) {
		for annotation := range pod.Annotations {
			if strings.Contains(annotation, "debug.openshift.io") {
				return "https://issues.redhat.com/browse/OCPBUGS-77201", true
			}
		}
		if managedBy, ok := pod.Labels["debug.openshift.io/managed-by"]; ok && managedBy == "oc-debug" {
			return "https://issues.redhat.com/browse/OCPBUGS-77201", true
		}
		return "", false
	},
}

// generateTestCases evaluates that no pods in the provided namespace are using the default service account.
// It returns the evaluated test cases or an error if any errors are encountered during the evaluation of the namespace.
func (n *noDefaultServiceAccountChecker) generateTestCases(ctx context.Context, namespace string) ([]*junitapi.JUnitTestCase, error) {
	podList, err := n.kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	testName := fmt.Sprintf("[sig-auth] all pods in %s namespace must not use the default service account.", namespace)
	junits := []*junitapi.JUnitTestCase{}
	exceptionList := []string{}
	failureList := []string{}
	for _, pod := range podList.Items {
		podSA := pod.Spec.ServiceAccountName

		// if the service account name is not default, we can exit for that iteration
		if podSA != "default" {
			continue
		}
		isException := false

		failure := fmt.Sprintf("pod %q is using the default service account", pod.Name)
		for _, exception := range exceptions {
			if msg, ok := exception(pod); ok {
				failure += fmt.Sprintf(" (exception: %s)", msg)
				exceptionList = append(exceptionList, failure)
				isException = true
				break
			}
		}

		if isException {
			continue
		}
		failureList = append(failureList, failure)
	}

	aggregatedList := append(failureList, exceptionList...)
	// if there is only passes for that ns
	if len(aggregatedList) == 0 {
		junits = append(junits, &junitapi.JUnitTestCase{Name: testName})
		return junits, nil
	}

	aggregatedListMsg := strings.Join(aggregatedList, "\n")

	junits = append(junits, &junitapi.JUnitTestCase{
		Name:          testName,
		SystemOut:     aggregatedListMsg,
		FailureOutput: &junitapi.FailureOutput{Output: aggregatedListMsg},
	})

	// if there are only exceptions we can add a flake
	if len(failureList) == 0 && len(exceptionList) != 0 {
		// introduce flake
		junits = append(junits, &junitapi.JUnitTestCase{
			Name: testName,
		})
	}

	return junits, nil
}

// CollectData implements monitortestframework.MonitorTest.
func (n *noDefaultServiceAccountChecker) CollectData(ctx context.Context, storageDir string, beginning time.Time, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if n.cfgClient == nil || n.kubeClient == nil {
		return nil, nil, nil
	}
	namespaces, err := n.kubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}
	junits := []*junitapi.JUnitTestCase{}
	for _, ns := range namespaces.Items {
		// Any namespaces with non-empty GenerateName attributes are dynamic namespace names.
		// These are exempt from testing as they cause CI Failures due to non static test naming.
		// Only known dynamic namespace is openshift-must-gather- which is exempt as it is not
		// core OpenShift.
		if ns.GenerateName != "" {
			continue
		}
		// We are only checking openshift- , kube- , and default namespaces as these are the namespaces which contain
		// core OpenShift components.
		if !strings.HasPrefix(ns.Name, "openshift-") && !strings.HasPrefix(ns.Name, "kube-") && ns.Name != "default" {
			continue
		}

		// use helper method to generate default service account failures
		testCases, err := n.generateTestCases(ctx, ns.Name)
		if err != nil {
			return nil, nil, err
		}
		junits = append(junits, testCases...)
	}
	return nil, junits, nil
}

// ConstructComputedIntervals implements monitortestframework.MonitorTest.
func (n *noDefaultServiceAccountChecker) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning time.Time, end time.Time) (constructedIntervals monitorapi.Intervals, err error) {
	return nil, nil
}

// EvaluateTestsFromConstructedIntervals implements monitortestframework.MonitorTest.
func (n *noDefaultServiceAccountChecker) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

// PrepareCollection implements monitortestframework.MonitorTest.
func (n *noDefaultServiceAccountChecker) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

// StartCollection implements monitortestframework.MonitorTest.
func (n *noDefaultServiceAccountChecker) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	var err error
	n.kubeClient, err = kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}
	n.cfgClient, err = configv1.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	return nil
}

// WriteContentToStorage implements monitortestframework.MonitorTest.
func (n *noDefaultServiceAccountChecker) WriteContentToStorage(ctx context.Context, storageDir string, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}
