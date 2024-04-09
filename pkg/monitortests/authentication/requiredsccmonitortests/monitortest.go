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

type requiredSCCAnnotationChecker struct {
	kubeClient kubernetes.Interface
}

func NewAnalyzer() monitortestframework.MonitorTest {
	return &requiredSCCAnnotationChecker{}
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
		// require that all workloads in openshift, kube-* or default namespaces must have the required-scc annotation
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
			if _, exists := pod.Annotations[securityv1.RequiredSCCAnnotation]; exists {
				continue
			}

			suggestedSCC := suggestSCC(&pod)
			owners := ownerReferences(&pod)
			failures = append(failures, fmt.Sprintf("annotation missing from pod '%s'%s; %s", pod.Name, owners, suggestedSCC))
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
			},

			// add a successful test with the same name to cause a flake
			&junitapi.JUnitTestCase{
				Name: testName,
			},
		)
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

// suggestSCC suggests the assigned SCC only if it belongs to the default set of SCCs
// pods in runlevel 0/1 namespaces won't have any assigned SCC as SCC admission is disabled
func suggestSCC(pod *v1.Pod) string {
	if len(pod.Annotations[securityv1.ValidatedSCCAnnotation]) == 0 {
		return "cannot suggest required-scc, no validated SCC on pod"
	}

	if defaultSCCs.Has(pod.Annotations[securityv1.ValidatedSCCAnnotation]) {
		return fmt.Sprintf("suggested required-scc: '%s'", pod.Annotations[securityv1.ValidatedSCCAnnotation])
	}

	return "cannot suggest required-scc, validated SCC is custom"
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
