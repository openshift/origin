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

// generateDefaultSAFailures generates a list of failures where the pod in a list of pods
// violated the default service account check.
func generateDefaultSAFailures(podList []corev1.Pod) []string {
	failures := make([]string, 0)
	for _, pod := range podList {
		podSA := pod.Spec.ServiceAccountName
		// if the service account name is not default, we can exit for that iteration
		fmt.Printf("Service account for pod %s in namespace %s is %s\n", pod.Name, pod.Namespace, pod.Spec.ServiceAccountName)
		if podSA != "default" {
			continue
		}
		// otherwise, we need to flag the failure
		failures = append(failures, fmt.Sprintf("service account name %s is being used in pod %s in namespace %s", podSA, pod.Name, pod.Namespace))
	}
	return failures
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
		if !strings.HasPrefix(ns.Name, "openshift") && !strings.HasPrefix(ns.Name, "kube-") && ns.Name != "default" {
			continue
		}

		// get list of all pods in the namespace
		pods, err := n.kubeClient.CoreV1().Pods(ns.Name).List(ctx, metav1.ListOptions{})

		if err != nil {
			return nil, nil, err
		}

		// use helper method to generate default service account failures
		failures := generateDefaultSAFailures(pods.Items)

		// generate tests for given namespace
		testName := fmt.Sprintf("[sig-auth] all operators in ns/%s must not use the 'default' service account", ns.Name)
		if len(failures) == 0 {
			junits = append(junits, &junitapi.JUnitTestCase{Name: testName})
			continue
		}
		failureMsg := strings.Join(failures, "\n")
		junits = append(junits, &junitapi.JUnitTestCase{
			Name:          testName,
			SystemOut:     failureMsg,
			FailureOutput: &junitapi.FailureOutput{Output: failureMsg},
		})
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
