package pdbunhealthypodevictionpolicy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var policyExceptions = map[string]sets.Set[string]{
	"openshift-etcd": sets.New("etcd-guard-pdb"),
}

type pdbUnhealthyPodEvictionPolicyChecker struct {
	kubeClient kubernetes.Interface
}

func NewAnalyzer() monitortestframework.MonitorTest {
	return &pdbUnhealthyPodEvictionPolicyChecker{}
}

func (w *pdbUnhealthyPodEvictionPolicyChecker) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	var err error
	w.kubeClient, err = kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	return nil
}

func (w *pdbUnhealthyPodEvictionPolicyChecker) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if w.kubeClient == nil {
		return nil, nil, nil
	}

	namespaces, err := w.kubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}

	junits := []*junitapi.JUnitTestCase{}
	for _, ns := range namespaces.Items {
		// ignore openshift-must-gather-* namespaces which are generated dynamically
		isPermanentOpenShiftNamespace := (ns.Name == "openshift" || strings.HasPrefix(ns.Name, "openshift-")) && !strings.HasPrefix(ns.Name, "openshift-must-gather-")
		if !isPermanentOpenShiftNamespace {
			continue
		}

		pdbs, err := w.kubeClient.PolicyV1().PodDisruptionBudgets(ns.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, nil, err
		}

		failures := make([]string, 0)
		for _, pdb := range pdbs.Items {
			if pdb.Spec.UnhealthyPodEvictionPolicy != nil && *pdb.Spec.UnhealthyPodEvictionPolicy == policyv1.AlwaysAllow {
				continue
			}
			if policyException, ok := policyExceptions[pdb.Namespace]; ok && policyException.Has(pdb.Name) {
				continue
			}

			failures = append(failures, fmt.Sprintf("AlwaysAllow .spec.unhealthyPodEvictionPolicy should be set on a %q PDB, found %q policy instead.", pdb.Name, ptr.Deref(pdb.Spec.UnhealthyPodEvictionPolicy, "<nil>")))
		}

		testName := fmt.Sprintf("[sig-apps] all PodDisruptionBudgets in ns/%s should have a %q .spec.unhealthyPodEvictionPolicy", ns.Name, policyv1.AlwaysAllow)
		if len(failures) == 0 {
			junits = append(junits, &junitapi.JUnitTestCase{Name: testName})
			continue
		}

		failureMsg := fmt.Sprintf("%v\n\n%v", strings.Join(failures, "\n"), PDBAlwaysAllowPolicyExplanation())
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

func (w *pdbUnhealthyPodEvictionPolicyChecker) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *pdbUnhealthyPodEvictionPolicyChecker) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (w *pdbUnhealthyPodEvictionPolicyChecker) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (w *pdbUnhealthyPodEvictionPolicyChecker) Cleanup(ctx context.Context) error {
	return nil
}

func PDBAlwaysAllowPolicyExplanation() string {
	return "AlwaysAllow policy allows eviction of unhealthy (not ready) pods even if there are no disruptions allowed on a PodDisruptionBudget." +
		" This can help to drain/maintain a node and recover without a manual intervention when multiple instances of nodes or pods are misbehaving." +
		" Use this with caution, as this option can disrupt perspective pods that have not yet had a chance to become healthy." +
		" See https://issues.redhat.com/browse/WRKLDS-1490 for more details and to assess whether creating an exception is warranted for the PDBs in question."
}
