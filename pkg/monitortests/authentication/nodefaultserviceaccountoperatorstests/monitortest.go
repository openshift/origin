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

		clusterOperatorsItems := []v1.ClusterOperator{} // used to filter on namespace

		for _, item := range clusterOperators.Items {
			if item.Namespace == ns.Name {
				clusterOperatorsItems = append(clusterOperatorsItems, item)
			}
		}

		// Grab the related objects within each cluster operator

		clusterOperatorsItemsRelatedObjects := [][]v1.ObjectReference{}

		for _, item := range clusterOperatorsItems {
			// append related objects for each operator in the list
			clusterOperatorsItemsRelatedObjects = append(clusterOperatorsItemsRelatedObjects, item.Status.RelatedObjects)
		}

		// create new list for pod items
		clusterOperatorObjects := []v1.ObjectReference{}

		// filter any objects that are pods using the Group ref. use later to perform lookup on pods service accounts.
		for _, item := range clusterOperatorsItemsRelatedObjects {
			// check the kind of the object (i.e. is it Pod ?)
			for _, item1 := range item {
				if item1.Group == "pods" {
					// add the item to the list
					clusterOperatorObjects = append(clusterOperatorObjects, item1)
				}
			}
		}

		// create a pod list
		podList := []corev1.Pod{}

		pods, err := n.kubeClient.CoreV1().Pods(ns.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, nil, err
		}

		// perform a lookup on the kubeclient to obtain all ACTUAL pod objects, which contain the `.Spec.ServiceAccountName` attribute.
		for _, object := range clusterOperatorObjects {
			for _, pod := range pods.Items {
				if pod.Name == object.Name && pod.Namespace == object.Namespace { // this check ensures uniqueness because a pod name MUST be unique in a given namespace
					podList = append(podList, pod)
				}
			}
		}

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
