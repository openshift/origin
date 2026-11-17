package psalabelchecker

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	psapi "k8s.io/pod-security-admission/api"
)

// exceptedNamespaces is the list of known namespaces where it has been found that they
// do not contain either some/all psa warn, audit or enforce labels, and/or they contain
// psa label syncer label.
// New offenders need a ticket filed under https://redhat.atlassian.net/CNTRLPLANE-4584.
var exceptedNamespaces = map[string]string{
	// these namespaces do not have some/all
	// pod-security.kubernetes.io/ labels set
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
	"openshift-node":                                   "CNTRLPLANE-4616",
	"openshift-route-controller-manager":               "CNTRLPLANE-4630",
	"openshift-service-ca":                             "CNTRLPLANE-4637",
	"openshift-service-ca-operator":                    "CNTRLPLANE-4637",
	"openshift-user-workload-monitoring":               "CNTRLPLANE-4638",
	"openshift-operator-lifecycle-manager":             "CNTRLPLANE-4636",
	"openshift-operators":                              "CNTRLPLANE-4636",
	// these namespaces set
	// security.openshift.io/scc.podSecurityLabelSync = false
	"openshift-cloud-platform-infra": "CNTRLPLANE-4848",
	"openshift-kni-infra":            "CNTRLPLANE-4848",
	"openshift-nutanix-infra":        "CNTRLPLANE-4848",
	"openshift-openstack-infra":      "CNTRLPLANE-4848",
	"openshift-ovirt-infra":          "CNTRLPLANE-4848",
	"openshift-vsphere-infra":        "CNTRLPLANE-4848",
	// caught outside testing - these namespaces set
	// security.openshift.io/scc.podSecurityLabelSync = true
	"openshift-kube-descheduler-operator":           "CNTRLPLANE-4987",
	"openshift-run-once-duration-override-operator": "CNTRLPLANE-4988",
}

// microshiftExceptedNamespaces covers namespaces MicroShift ships its own manifests
// for; those need fixing separately from standalone, hence a separate ticket.
var microshiftExceptedNamespaces = map[string]string{
	"openshift-controller-manager":       "CNTRLPLANE-4751",
	"openshift-route-controller-manager": "CNTRLPLANE-4751",
	"openshift-service-ca":               "CNTRLPLANE-4751",
}

// hypershiftExceptedNamespaces covers namespaces HyperShift ships its own manifests
// for; those need fixing separately from standalone, hence a separate ticket. Anything
// not listed comes from a bundled image and stays under exceptedNamespaces.
var hypershiftExceptedNamespaces = map[string]string{
	"openshift-cloud-controller-manager": "CNTRLPLANE-4825",
	"openshift-apiserver":                "CNTRLPLANE-4825",
	"openshift-controller-manager":       "CNTRLPLANE-4825",
	"openshift-kube-apiserver":           "CNTRLPLANE-4825",
	"openshift-kube-controller-manager":  "CNTRLPLANE-4825",
	"openshift-kube-scheduler":           "CNTRLPLANE-4825",
	"openshift-etcd":                     "CNTRLPLANE-4825",
	"openshift-authentication":           "CNTRLPLANE-4825",
	"openshift-route-controller-manager": "CNTRLPLANE-4825",
}

// psaViolationRegex is used later to check dry-run warnings for
// namespace violations.
var psaViolationRegex = regexp.MustCompile(`existing pods in namespace "([^"]+)" violate the new PodSecurity enforce level "([^"]+)"`)

type noPodSecurityAdmissionLabelNamespaceChecker struct {
	kubeClient   kubernetes.Interface
	configClient configv1client.Interface
	wngCollector *warningCollector
}

// Cleanup implements monitortestframework.MonitorTest
func (n *noPodSecurityAdmissionLabelNamespaceChecker) Cleanup(ctx context.Context) error {
	return nil
}

// generateTestCase asserts that an OpenShift managed namespace sets the enforce, audit and
// warn pod-security.kubernetes.io/ labels and no longer opts into the PSA label syncer.
// Both are prerequisites for retiring the syncer without weakening PSA enforcement.
func (n *noPodSecurityAdmissionLabelNamespaceChecker) generateTestCase(namespace corev1.Namespace, isMicroShift bool, isHyperShift bool) []*junitapi.JUnitTestCase {
	if n.configClient == nil || n.kubeClient == nil || n.wngCollector == nil {
		return nil
	}

	nsCopy := namespace.DeepCopy()

	namespaceName := nsCopy.Name

	testName := fmt.Sprintf("[sig-auth] namespace %s must set the pod security admission level labels", namespaceName)

	if nsCopy.Labels == nil {
		nsCopy.Labels = make(map[string]string)
	}

	problems := []string{}
	for _, key := range []string{psapi.EnforceLevelLabel, psapi.AuditLevelLabel, psapi.WarnLevelLabel} {
		_, ok := nsCopy.Labels[key]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s is not set", key))
			continue
		}
	}

	// The namespace needs to be checked if it can enforce restricted PSS.
	// This check occurs only if the namespace does not have enforce label.
	// This can be safely done with a dry-run update with `restricted` PSS
	// enforcement applied.
	_, ok := nsCopy.Labels[psapi.EnforceLevelLabel]
	if !ok {
		nsCopy.Labels[psapi.EnforceLevelLabel] = string(psapi.LevelRestricted)
		nsCopy.Labels[psapi.EnforceVersionLabel] = psapi.LatestVersion().String()

		_, err := n.kubeClient.CoreV1().Namespaces().Update(
			context.Background(),
			nsCopy,
			metav1.UpdateOptions{DryRun: []string{metav1.DryRunAll}},
		)

		if err != nil {
			problems = append(problems, fmt.Sprintf("namespace %s failed to update with dry run: %v", nsCopy.Name, err))
		}

		warnings := n.wngCollector.PopWarnings()

		for _, warning := range warnings {
			if psaViolationRegex.MatchString(warning) {
				problems = append(problems, fmt.Sprintf("namespace %s cannot enforce restricted PSS: %s", nsCopy.Name, warning))
			}
		}
	}

	// reset nsCopy since we modified its labels...
	nsCopy = namespace.DeepCopy()

	// The syncer opt-in becomes a no-op once the syncer is disabled, so it has to be
	// cleaned up rather than left behind as a false signal.
	sccPsaLabelSyncPrefix := "security.openshift.io/scc.podSecurityLabelSync"
	sccPsaLabelSyncValue := nsCopy.Labels[sccPsaLabelSyncPrefix]
	if sccPsaLabelSyncValue == "false" {
		problems = append(problems, fmt.Sprintf("%s is set to %s and needs to be removed", sccPsaLabelSyncPrefix, sccPsaLabelSyncValue))
	}

	// We must flag when sccPsaLabelSyncValue is true, as urgent.
	// Even if pod-security.kubernetes.io/enforce is set, the PSa
	// label syncer will override these values.
	// ** Before the PSa label syncer is disabled, these exceptions
	// must be dealt with to avoid unexpected workload failures. **
	if sccPsaLabelSyncValue == "true" {
		problems = append(problems, fmt.Sprintf("[URGENT] %s is set to %s and needs to be removed", sccPsaLabelSyncPrefix, sccPsaLabelSyncValue))
	}

	if len(problems) == 0 {
		return []*junitapi.JUnitTestCase{{Name: testName}}
	}

	failure := fmt.Sprintf("namespace %q does not set the pod security admission level labels: %s", namespaceName, strings.Join(problems, ", "))

	reason, isException := exceptedNamespaces[namespaceName]

	// Prefer the platform-specific ticket, since that manifest is what needs fixing.
	if hypershiftReason, isHyperShiftException := hypershiftExceptedNamespaces[namespaceName]; isHyperShift && isHyperShiftException {
		reason, isException = hypershiftReason, true
	}

	if microshiftReason, isMicroShiftException := microshiftExceptedNamespaces[namespaceName]; isMicroShift && isMicroShiftException {
		reason, isException = microshiftReason, true
	}

	// Emitted as a flake (failure + success) while managed namespaces are still being
	// labelled; failing hard today would fail every job. Drop the trailing success case
	// to make this a real failure.
	if isException {
		exceptionMsg := fmt.Sprintf("namespace %q has an exception in place. See https://redhat.atlassian.net/browse/%s for reference. ", namespaceName, reason)
		return []*junitapi.JUnitTestCase{
			{
				Name:          testName,
				SystemOut:     failure,
				FailureOutput: &junitapi.FailureOutput{Output: exceptionMsg + failure},
			},
			{
				Name:      testName,
				SystemOut: exceptionMsg,
			},
		}
	}
	return []*junitapi.JUnitTestCase{
		{
			Name:          testName,
			SystemOut:     failure,
			FailureOutput: &junitapi.FailureOutput{Output: failure},
		},
		// TODO(ehearne-redhat): drop once post-merge runs confirm every exception is
		// captured above. Post-merge jobs surfaced uncaught ones for the default service
		// account monitor test.
		{
			Name:      testName,
			SystemOut: "Passing the case to make the overall test case flake while managed namespaces are being labelled",
		},
	}
}

// CollectData implements monitortestframework.MonitorTest
func (n *noPodSecurityAdmissionLabelNamespaceChecker) CollectData(ctx context.Context, storageDir string, beginning time.Time, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if n.configClient == nil || n.kubeClient == nil || n.wngCollector == nil {
		return nil, nil, nil
	}
	namespaces, err := n.kubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}

	isHyperShift, _ := exutil.IsHypershift(ctx, n.configClient)
	isMicroShift, _ := exutil.IsMicroShiftCluster(n.kubeClient)

	junits := []*junitapi.JUnitTestCase{}
	for _, ns := range namespaces.Items {
		// Dynamic names would produce non-static test names and break CI reporting.
		if ns.GenerateName != "" {
			continue
		}

		podList, err := n.kubeClient.CoreV1().Pods(ns.Name).List(ctx, metav1.ListOptions{})

		if err != nil {
			return nil, nil, err
		}
		// We can ignore namespaces with no workloads
		if len(podList.Items) == 0 {
			continue
		}

		// Annotated namespaces are shipped to, but not created by, HyperShift.
		// By including this check, we help to reduce duplicate reportings of
		// namespace manifests that do not belong to HyperShift, but rather they
		// belong to an OCP component repo instead.
		isHyperShiftIncludedNamespace := ns.Annotations["include.release.openshift.io/hypershift"] == "true"

		// Under HyperShift these are created by openshift-apiserver, not HyperShift itself,
		// and are tracked under CNTRLPLANE-4616 instead.
		isOpenShiftAPIServerManaged := ns.Name == "openshift" || ns.Name == "openshift-node"

		_, isException := exceptedNamespaces[ns.Name]

		if isHyperShift && (isHyperShiftIncludedNamespace || isOpenShiftAPIServerManaged) && isException {
			continue
		}

		isManagedNamespace := ns.Name == "openshift" || strings.HasPrefix(ns.Name, "openshift-")
		if !isManagedNamespace {
			continue
		}
		junits = append(junits, n.generateTestCase(ns, isMicroShift, isHyperShift)...)
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
	cfg := rest.CopyConfig(adminRESTConfig)
	n.wngCollector = &warningCollector{}
	cfg.WarningHandler = n.wngCollector

	var err error

	n.kubeClient, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}

	n.configClient, err = configv1client.NewForConfig(cfg)
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

type warningCollector struct {
	mu       sync.Mutex
	warnings []string
}

// HandleWarningHeader is invoked by client-go whenever the API server returns a "Warning" HTTP header.
func (w *warningCollector) HandleWarningHeader(code int, agent string, text string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.warnings = append(w.warnings, text)
}

func (w *warningCollector) PopWarnings() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := w.warnings
	w.warnings = nil
	return out
}
