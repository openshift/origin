package requiredsccmonitortests

import (
	"context"
	"fmt"
	"strings"
	"time"

	securityv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var defaultSCCs = sets.NewString(
	"anyuid",
	"hostaccess",
	"hostmount-anyuid",
	"hostnetwork",
	"hostnetwork-v2",
	"nonroot",
	"nonroot-v2",
	"privileged",
	"restricted",
	"restricted-v2",
)

var nonStandardSCCNamespaces = map[string]sets.Set[string]{
	"node-exporter":                   sets.New("openshift-monitoring"),
	"machine-api-termination-handler": sets.New("openshift-machine-api"),
}

// namespacesWithPendingSCCPinning includes namespaces with workloads that have pending SCC pinning.
var namespacesWithPendingSCCPinning = sets.NewString(
	"openshift-cluster-csi-drivers",
	"openshift-cluster-version",
	"openshift-image-registry",
	"openshift-ingress",
	"openshift-ingress-canary",
	"openshift-ingress-operator",
	"openshift-insights",
	"openshift-machine-api",
	"openshift-monitoring",
	// run-level namespaces
	"openshift-cloud-controller-manager",
	"openshift-cloud-controller-manager-operator",
	"openshift-cluster-api",
	"openshift-cluster-machine-approver",
	"openshift-dns",
	"openshift-dns-operator",
	"openshift-etcd",
	"openshift-etcd-operator",
	"openshift-kube-apiserver",
	"openshift-kube-apiserver-operator",
	"openshift-kube-controller-manager",
	"openshift-kube-controller-manager-operator",
	"openshift-kube-proxy",
	"openshift-kube-scheduler",
	"openshift-kube-scheduler-operator",
	"openshift-multus",
	"openshift-network-operator",
	"openshift-ovn-kubernetes",
	"openshift-sdn",
	"openshift-storage",
)

// systemNamespaces includes namespaces that should be treated as flaking.
// these namespaces are included because we don't control their creation or labeling on their creation.
var systemNamespaces = sets.NewString(
	"default",
	"kube-system",
	"kube-public",
	"openshift-node",
	"openshift-infra",
	"openshift",
)

type requiredSCCAnnotationChecker struct {
	kubeClient kubernetes.Interface
}

func NewAnalyzer() monitortestframework.MonitorTest {
	return &requiredSCCAnnotationChecker{}
}

func (w *requiredSCCAnnotationChecker) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *requiredSCCAnnotationChecker) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	var err error
	w.kubeClient, err = kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	return nil
}

func (w *requiredSCCAnnotationChecker) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if w.kubeClient == nil {
		return nil, nil, nil
	}

	namespaces, err := w.kubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}

	junits := []*junitapi.JUnitTestCase{}
	for _, ns := range namespaces.Items {
		// skip managed service namespaces
		if exutil.ManagedServiceNamespaces.Has(ns.Name) {
			continue
		}

		// require that all workloads in openshift, kube-*, or default namespaces must have the required-scc annotation
		// ignore openshift-must-gather-* namespaces which are generated dynamically
		isPermanentOpenShiftNamespace := (ns.Name == "openshift" || strings.HasPrefix(ns.Name, "openshift-")) && !strings.HasPrefix(ns.Name, "openshift-must-gather-")
		if !strings.HasPrefix(ns.Name, "kube-") && ns.Name != "default" && !isPermanentOpenShiftNamespace {
			continue
		}

		pods, err := w.kubeClient.CoreV1().Pods(ns.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, nil, err
		}

		failures := make([]string, 0)
		for _, pod := range pods.Items {
			validatedSCC := pod.Annotations[securityv1.ValidatedSCCAnnotation]
			allowedNamespaces, isNonStandard := nonStandardSCCNamespaces[validatedSCC]

			if _, exists := pod.Annotations[securityv1.RequiredSCCAnnotation]; exists {
				if isNonStandard && !allowedNamespaces.Has(ns.Name) {
					failures = append(failures, fmt.Sprintf(
						"pod '%s' has a non-standard SCC '%s' not allowed in namespace '%s'; allowed namespaces are: %s",
						pod.Name, validatedSCC, ns.Name, strings.Join(allowedNamespaces.UnsortedList(), ", ")))
				}
				continue
			}

			owners := ownerReferences(&pod)

			switch {
			case len(validatedSCC) == 0:
				failures = append(failures, fmt.Sprintf(
					"annotation missing from pod '%s'%s; cannot suggest required-scc, no validated SCC on pod",
					pod.Name, owners))

			case defaultSCCs.Has(validatedSCC):
				failures = append(failures, fmt.Sprintf(
					"annotation missing from pod '%s'%s; suggested required-scc: '%s'",
					pod.Name, owners, validatedSCC))

			case isNonStandard:
				if allowedNamespaces.Has(ns.Name) {
					failures = append(failures, fmt.Sprintf(
						"annotation missing from pod '%s'%s; suggested required-scc: '%s', this is a non-standard SCC",
						pod.Name, owners, validatedSCC))
				} else {
					failures = append(failures, fmt.Sprintf(
						"annotation missing from pod '%s'%s; pod is using non-standard SCC '%s' not allowed in namespace '%s'; allowed namespaces are: %s",
						pod.Name, owners, validatedSCC, ns.Name, strings.Join(allowedNamespaces.UnsortedList(), ", ")))
				}

			default:
				failures = append(failures, fmt.Sprintf(
					"annotation missing from pod '%s'%s; cannot suggest required-scc, validated SCC '%s' is a custom SCC",
					pod.Name, owners, validatedSCC))
			}
		}

		testName := fmt.Sprintf("[sig-auth] all workloads in ns/%s must set the '%s' annotation", ns.Name, securityv1.RequiredSCCAnnotation)
		if len(failures) == 0 {
			junits = append(junits, &junitapi.JUnitTestCase{Name: testName})
			continue
		}

		failureMsg := strings.Join(failures, "\n")

		junits = append(junits,
			&junitapi.JUnitTestCase{
				Name:          testName,
				SystemOut:     failureMsg,
				FailureOutput: &junitapi.FailureOutput{Output: failureMsg},
			})

		// add a successful test with the same name to cause a flake if the namespace should be flaking
		if namespacesWithPendingSCCPinning.Has(ns.Name) || systemNamespaces.Has(ns.Name) {
			junits = append(junits,
				&junitapi.JUnitTestCase{
					Name: testName,
				})
		}
	}

	return nil, junits, nil
}

func (w *requiredSCCAnnotationChecker) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *requiredSCCAnnotationChecker) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (w *requiredSCCAnnotationChecker) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (w *requiredSCCAnnotationChecker) Cleanup(ctx context.Context) error {
	return nil
}

func ownerReferences(pod *v1.Pod) string {
	ownerRefs := make([]string, len(pod.OwnerReferences))
	for i, or := range pod.OwnerReferences {
		ownerRefs[i] = fmt.Sprintf("%s/%s", strings.ToLower(or.Kind), or.Name)
	}

	if len(ownerRefs) > 0 {
		return fmt.Sprintf(" (owners: %s)", strings.Join(ownerRefs, ", "))
	}

	return ""
}
