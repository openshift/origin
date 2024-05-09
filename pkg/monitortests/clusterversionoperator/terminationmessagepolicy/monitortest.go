package terminationmessagepolicy

import (
	"context"
	"fmt"
	"strings"
	"time"

	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	unfixedVersions = sets.NewString()
)

func init() {
	for i := 0; i < 16; i++ {
		unfixedVersions.Insert(fmt.Sprintf("4.%d", i))
	}
}

type terminationMessagePolicyChecker struct {
	kubeClient    kubernetes.Interface
	hasOldVersion bool
}

func NewAnalyzer() monitortestframework.MonitorTest {
	return &terminationMessagePolicyChecker{}
}

func (w *terminationMessagePolicyChecker) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	var err error
	w.kubeClient, err = kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}
	configClient, err := configclient.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}
	clusterVersion, err := configClient.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
	if err != nil {
		return err
	}

	for _, history := range clusterVersion.Status.History {
		for _, unfixedVersion := range unfixedVersions.List() {
			if strings.Contains(history.Version, unfixedVersion) {
				w.hasOldVersion = true
				break
			}
		}
		if w.hasOldVersion {
			break
		}
	}

	return nil
}

func (w *terminationMessagePolicyChecker) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if w.kubeClient == nil {
		return nil, nil, nil
	}

	allPods, err := w.kubeClient.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}

	failuresByNamespace := map[string][]string{}
	for _, pod := range allPods.Items {
		if !strings.HasPrefix(pod.Namespace, "openshift") {
			continue
		}
		// skip generated platform namespaces
		if strings.HasPrefix(pod.Namespace, "openshift-must-gather") {
			continue
		}
		if _, ok := failuresByNamespace[pod.Namespace]; !ok {
			failuresByNamespace[pod.Namespace] = []string{}
		}

		for _, container := range pod.Spec.InitContainers {
			if container.TerminationMessagePolicy != corev1.TerminationMessageFallbackToLogsOnError {
				failuresByNamespace[pod.Namespace] = append(failuresByNamespace[pod.Namespace],
					fmt.Sprintf("pods/%s initContainers[%v]", pod.Name, container.Name))
			}
		}
		for _, container := range pod.Spec.Containers {
			if container.TerminationMessagePolicy != corev1.TerminationMessageFallbackToLogsOnError {
				failuresByNamespace[pod.Namespace] = append(failuresByNamespace[pod.Namespace],
					fmt.Sprintf("pods/%s containers[%v]", pod.Name, container.Name))

			}
		}
		for _, container := range pod.Spec.EphemeralContainers {
			if container.TerminationMessagePolicy != corev1.TerminationMessageFallbackToLogsOnError {
				failuresByNamespace[pod.Namespace] = append(failuresByNamespace[pod.Namespace],
					fmt.Sprintf("pods/%s ephemeralContainers[%v]", pod.Name, container.Name))

			}
		}
	}

	// existingViolations is the list of violations already present, don't add to it once we start enforcing
	existingViolations := map[string]sets.String{
		"namespace": sets.NewString("<containerType>[<containerName>]"),
		"openshift-catalogd": sets.NewString(
			"pods/catalogd-controller-manager",
		),
		"openshift-cluster-api": sets.NewString(
			"pods/capa-controller-manager",
			"pods/capi-controller-manager",
		),
		"openshift-cluster-csi-drivers": sets.NewString(
			"containers[openstack-cinder-csi-driver-operator]",
			"containers[azure-file-csi-driver-operator]",
			"containers[shared-resource-csi-driver-operator]",
			"containers[vmware-vsphere-csi-driver-operator]",
			"containers[vsphere-webhook]",
			"pods/kubevirt-csi-node",
			"pods/nutanix-csi-controller",
			"pods/nutanix-csi-node",
			"pods/ibm-vpc-block-csi-controller",
			"pods/ibm-vpc-block-csi-node",
			"pods/powervs-block-csi-driver-operator",
		),
		"openshift-cloud-controller-manager": sets.NewString(
			"pods/aws-cloud-controller-manager",
		),
		"openshift-machine-api": sets.NewString(
			"containers[machine-ipam-controller]",
		),
		"openshift-multus": sets.NewString(
			"containers[multus-networkpolicy]",
		),
	}

	junits := []*junitapi.JUnitTestCase{}
	for _, namespace := range sets.StringKeySet(failuresByNamespace).List() {
		testName := fmt.Sprintf("[sig-arch] all containers in ns/%v must have terminationMessagePolicy=%v", namespace, corev1.TerminationMessageFallbackToLogsOnError)
		failingContainers := sets.NewString(failuresByNamespace[namespace]...)
		if len(failingContainers) == 0 {
			junits = append(junits, &junitapi.JUnitTestCase{
				Name:      testName,
				SystemOut: "",
				SystemErr: "",
			})
			continue
		}

		flakyContainers := sets.NewString()
		existingViolationForNamespace := existingViolations[namespace]
		for _, failingContainer := range failingContainers.List() {
			found := false
			for _, existingViolatingContainer := range existingViolationForNamespace.List() {
				if strings.Contains(failingContainer, existingViolatingContainer) {
					found = true
				}
			}
			if found || w.hasOldVersion {
				// if we have an existing violation or an older (unfixed) level of openshift was installed
				// we need to skip the offending container and only flake (not fail) on it.
				flakyContainers.Insert(failingContainer)
				failingContainers.Delete(failingContainer)
			}
		}

		// if we have flakes, add flakes as a junit failure.
		if len(flakyContainers) > 0 {
			failureMessages := []string{}
			for _, container := range flakyContainers.List() {
				failureMessages = append(failureMessages,
					fmt.Sprintf("%v should have terminationMessagePolicy=%q",
						container, corev1.TerminationMessageFallbackToLogsOnError))
			}
			junits = append(junits, &junitapi.JUnitTestCase{
				Name:      testName,
				SystemOut: strings.Join(failureMessages, "\n"),
				FailureOutput: &junitapi.FailureOutput{
					Output: strings.Join(failureMessages, "\n"),
				},
			})
		}

		// if we had no failures (only flakes or zero flakes), add a successful test (either succeeds or flakes) and return
		if len(failingContainers) == 0 {
			junits = append(junits, &junitapi.JUnitTestCase{
				Name:      testName,
				SystemOut: "",
				SystemErr: "",
			})
			continue
		}

		// add failures as a failing junit
		failureMessages := []string{}
		for _, container := range failingContainers.List() {
			failureMessages = append(failureMessages,
				fmt.Sprintf("%v must have terminationMessagePolicy=%q",
					container, corev1.TerminationMessageFallbackToLogsOnError))
		}
		junits = append(junits,
			&junitapi.JUnitTestCase{
				Name:      testName,
				SystemOut: strings.Join(failureMessages, "\n"),
				FailureOutput: &junitapi.FailureOutput{
					Output: strings.Join(failureMessages, "\n"),
				},
			},
		)

	}

	return nil, junits, nil
}

func (*terminationMessagePolicyChecker) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (*terminationMessagePolicyChecker) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (*terminationMessagePolicyChecker) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*terminationMessagePolicyChecker) Cleanup(ctx context.Context) error {
	return nil
}
