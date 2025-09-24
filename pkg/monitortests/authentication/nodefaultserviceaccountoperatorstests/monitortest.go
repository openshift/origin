package nodefaultserviceaccountoperatortests

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1 "github.com/openshift/api/config/v1"
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

// fetchRelatedObjects returns a list of RelatedObjects found within a specified
// namespace for a given list of cluster operators .
func fetchPodObjects(clusterOperators *v1.ClusterOperatorList, ns corev1.Namespace) []v1.ObjectReference {
	// create new list for pod items (object references to pods)
	clusterOperatorPodObjects := []v1.ObjectReference{}
	// filter any objects that are pods using the Group ref. use later to perform lookup on pods service accounts.
	for _, item := range clusterOperators.Items {
		if item.Namespace != ns.Namespace {
			continue
		}
		// check the kind of the object (i.e. is it Pod ?)
		for _, item1 := range item.Status.RelatedObjects {
			if item1.Group == "pods" {
				// add the item to the list
				clusterOperatorPodObjects = append(clusterOperatorPodObjects, item1)
			}
		}
	}
	return clusterOperatorPodObjects
}

// fetchPodList returns a list of pods from a list of pod related objects based on a list of pods
// retrieved elsewhere - in our case, from a namespace.
func fetchPodList(clusterOperatorPodObjects []v1.ObjectReference, pods corev1.PodList) []corev1.Pod {
	// create a pod list
	podList := []corev1.Pod{}

	// create a pod map
	podMap := map[string]corev1.Pod{}

	// create an object map
	objectMap := map[string]v1.ObjectReference{}

	// add pod items to pod map
	for _, pod := range pods.Items {
		podMap[pod.Name] = pod
	}

	// add object items to object map
	for _, object := range clusterOperatorPodObjects {
		objectMap[object.Name] = object
	}

	// perform a lookup on the kubeclient to obtain all ACTUAL pod objects, which contain the `.Spec.ServiceAccountName` attribute.
	for _, pod := range podMap {
		// this check ensures uniqueness because a pod name MUST be unique in a given namespace
		if podMap[pod.Name].Name == objectMap[pod.Name].Name &&
			podMap[pod.Name].Namespace == objectMap[pod.Name].Namespace {
			podList = append(podList, pod)
		}
	}

	return podList
}

// generateDefaultSAFailures generates a list of failures where the pod in a list of pods
// violated the default service account check.
func generateDefaultSAFailures(podList []corev1.Pod) []string {
	failures := make([]string, 0)
	for _, pod := range podList {
		podSA := pod.Spec.ServiceAccountName
		// if the service account name is not default, we can exit for that iteration
		if podSA != "default" {
			continue
		}
		// otherwise, we need to flag the failure
		failures = append(failures, fmt.Sprintf("service account name %s is being used in pod %s", podSA, pod.Name))
	}
	return failures
}

// CollectData implements monitortestframework.MonitorTest.
func (n *noDefaultServiceAccountChecker) CollectData(ctx context.Context, storageDir string, beginning time.Time, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {

	if n.cfgClient == nil || n.kubeClient == nil {
		return nil, nil, nil
	}

	clusterOperators, err := n.cfgClient.ClusterOperators().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
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

		// use helper method to fetch pod related objects for cluster operators in the current ns iteration.
		clusterOperatorPodObjects := fetchPodObjects(clusterOperators, ns)

		// get list of all pods in the namespace
		pods, err := n.kubeClient.CoreV1().Pods(ns.Name).List(ctx, metav1.ListOptions{})

		if err != nil {
			return nil, nil, err
		}

		// use helper method to fetch pod list for pod related objects
		podList := fetchPodList(clusterOperatorPodObjects, *pods)

		// use helper method to generate default service account failures
		failures := generateDefaultSAFailures(podList)

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
