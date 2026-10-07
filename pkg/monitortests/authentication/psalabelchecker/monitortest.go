package psalabelchecker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	psapi "k8s.io/pod-security-admission/api"
)

// exceptedNamespaces are OpenShift managed namespaces that have been found in
// monitor testing so far. Ideally, if a long-standing namespace has not been
// included, a ticket should be filed and attached to CNTRLPLANE-4584.
// See any ticket attached to https://redhat.atlassian.net/CNTRLPLANE-4584
// for reference.

// The list follows this convention -
//
//	{
//	  ..
//	  "<namespace>":"<exception ticket identifier>",
//	  ..
//	}
var exceptedNamespaces = map[string]string{
	"openshift":                                        "CNTRLPLANE-4616",
	"openshift-apiserver-operator":                     "CNTRLPLANE-4617",
	"openshift-cloud-credential-operator":              "CNTRLPLANE-4618",
	"openshift-cloud-network-config-controller":        "CNTRLPLANE-4620",
	"openshift-cluster-olm-operator":                   "CNTRLPLANE-4621",
	"openshift-cluster-samples-operator":               "CNTRLPLANE-4622",
	"openshift-cluster-storage-operator":               "CNTRLPLANE-4623",
	"openshift-config-managed":                         "CNTRLPLANE-4627",
	"openshift-config-operator":                        "CNTRLPLANE-4627",
	"openshift-console":                                "CNTRLPLANE-4628",
	"openshift-console-operator":                       "CNTRLPLANE-4628",
	"openshift-console-user-settings":                  "CNTRLPLANE-4628",
	"openshift-controller-manager":                     "CNTRLPLANE-4630",
	"openshift-controller-manager-operator":            "CNTRLPLANE-4630",
	"openshift-dns-operator":                           "CNTRLPLANE-4631",
	"openshift-host-network":                           "CNTRLPLANE-4620",
	"openshift-ingress-canary":                         "CNTRLPLANE-4633",
	"openshift-ingress-operator":                       "CNTRLPLANE-4633",
	"openshift-kube-controller-manager-operator":       "CNTRLPLANE-4634",
	"openshift-kube-storage-version-migrator":          "CNTRLPLANE-4635",
	"openshift-kube-storage-version-migrator-operator": "CNTRLPLANE-4635",
	"openshift-network-console":                        "CNTRLPLANE-4620",
	"openshift-network-diagnostics":                    "CNTRLPLANE-4620",
	"openshift-network-node":                           "CNTRLPLANE-4616",
	"openshift-route-controller-manager":               "CNTRLPLANE-4630",
	"openshift-service-ca":                             "CNTRLPLANE-4637",
	"openshift-service-ca-operator":                    "CNTRLPLANE-4637",
	"openshift-user-workload-monitoring":               "CNTRLPLANE-4638",
	"openshift-operator-lifecycle-manager":             "CNTRLPLANE-4636",
	"openshift-operators":                              "CNTRLPLANE-4636",
}

type noPodSecurityAdmissionLabelNamespaceChecker struct {
	kubeClient kubernetes.Interface
}

// Cleanup implements monitortestframework.MonitorTest
func (n *noPodSecurityAdmissionLabelNamespaceChecker) Cleanup(ctx context.Context) error {
	return nil
}

// generateTestCase checks to see if a given namespace has the appropriate PSA labels (pod-security.kubernetes.io/) and
// whether the PSA Label Syncer label (security.openshift.io/scc.podSecurityLabelSync) is in use for that namespace.
// The namespaces that end up being checked here should be managed by OpenShift. Ideally, these namespaces will set the
// `enforce`, `audit` and `warn` PSA labels to `restricted` so that they do not interfere with the global PSA enforcement,
// and the PSA Label Syncer label should be removed as the managed namespaces eventually become appropriately labelled.
func generateTestCase(namespace corev1.Namespace) []*junitapi.JUnitTestCase {
	namespaceName := namespace.Name

	testName := fmt.Sprintf("[sig-auth] namespace %s must set the pod security admission level labels", namespaceName)

	problems := []string{}
	for _, key := range []string{psapi.EnforceLevelLabel, psapi.AuditLevelLabel, psapi.WarnLevelLabel} {
		if _, ok := namespace.Labels[key]; !ok {
			problems = append(problems, fmt.Sprintf("%s is not set", key))
		}
	}

	// In order to disable PSA Label Syncer, we need to keep track of and
	// remove remnant annotations that tell PSA Label Syncer to label the
	// namespace. This is because the annotation will be rendered useless
	// upon disablement. All OpenShift namespaces should be labelled with
	// pod-security.kubernetes.io/ enforce, audit and warn labels so that
	// this can happen safely.
	sccPsaLabelSyncPrefix := "security.openshift.io/scc.podSecurityLabelSync"
	if _, ok := namespace.Labels[sccPsaLabelSyncPrefix]; ok {
		problems = append(problems, "%s is set and needs to be removed", sccPsaLabelSyncPrefix)
	}

	if len(problems) == 0 {
		return []*junitapi.JUnitTestCase{{Name: testName}}
	}

	failure := fmt.Sprintf("namespace %q does not set the pod security admission level labels: %s", namespaceName, strings.Join(problems, ", "))
	// Reported as a flake while managed namespaces are being labelled ahead of the PSA label
	// syncer retirement. Many are still unlabelled, so failing hard would fail every job.
	// Remove the trailing success case to turn this into a hard failure.
	if reason, isException := exceptedNamespaces[namespaceName]; isException {
		return []*junitapi.JUnitTestCase{
			{
				Name:          testName,
				SystemOut:     failure,
				FailureOutput: &junitapi.FailureOutput{Output: failure},
			},
			{
				Name:      testName,
				SystemOut: fmt.Sprintf("namespace %q has an exception in place. See https://redhat.atlassian.net/browse/%s for reference.", namespaceName, reason),
			},
		}
	}
	return []*junitapi.JUnitTestCase{
		{
			Name:          testName,
			SystemOut:     failure,
			FailureOutput: &junitapi.FailureOutput{Output: failure},
		},
		{
			Name:      testName,
			SystemOut: "Passing the case to make the overall test case flake while managed namespaces are being labelled",
		},
	}
}

// CollectData implements monitortestframework.MonitorTest
func (n *noPodSecurityAdmissionLabelNamespaceChecker) CollectData(ctx context.Context, storageDir string, beginning time.Time, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if n.kubeClient == nil {
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
		if ns.GenerateName != "" {
			continue
		}
		// We are only checking OpenShift managed namespaces.
		isManagedNamespace := ns.Name == "openshift" || strings.HasPrefix(ns.Name, "openshift-")
		if !isManagedNamespace {
			continue
		}
		junits = append(junits, generateTestCase(ns)...)
	}
	return nil, junits, nil
}

// ConstructComputedIntervals implements monitortestframework.MonitorTest.
func (n *noPodSecurityAdmissionLabelNamespaceChecker) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning time.Time, end time.Time) (constructedIntervals monitorapi.Intervals, err error) {
	return nil, nil
}

// EvaluateTestsFromConstructedIntervals implements monitortestframework.MonitorTest.
func (n *noPodSecurityAdmissionLabelNamespaceChecker) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

// PrepareCollection implements monitortestframework.MonitorTest.
func (n *noPodSecurityAdmissionLabelNamespaceChecker) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

// StartCollection implements monitortestframework.MonitorTest.
func (n *noPodSecurityAdmissionLabelNamespaceChecker) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	var err error
	n.kubeClient, err = kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	return nil
}

// WriteContentToStorage implements monitortestframework.MonitorTest.
func (n *noPodSecurityAdmissionLabelNamespaceChecker) WriteContentToStorage(ctx context.Context, storageDir string, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func NewAnalyzer() monitortestframework.MonitorTest {
	return &noPodSecurityAdmissionLabelNamespaceChecker{}
}
