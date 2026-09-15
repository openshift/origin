package nopodsecurityadmissionlabelnamespacetests

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

// exceptedNamespaces maps an OpenShift managed namespace to the reason it is exempt from the pod
// security admission label check.
var exceptedNamespaces = map[string]string{
	"openshift-operator-lifecycle-manager": "OLM-owned namespace, labelled by OLM rather than by a payload manifest",
	"openshift-operators":                  "OLM installs arbitrary operator workloads here, so the level is not fixed by a payload manifest",
}

type noPodSecurityAdmissionLabelNamespaceChecker struct {
	kubeClient kubernetes.Interface
	cfgClient  *configv1.ConfigV1Client
}

// Cleanup implements monitortestframework.MonitorTest
func (n *noPodSecurityAdmissionLabelNamespaceChecker) Cleanup(ctx context.Context) error {
	return nil
}

// generateTestCase evaluates that an OpenShift managed namespace sets the three pod security
// admission level labels. The level itself is not checked — any value is acceptable — because the
// goal is that every managed namespace carries an explicit level before the PSA label syncer is
// retired, at which point an unlabelled namespace silently falls back to the cluster default.
// Any further PSA label a namespace sets is left alone. Namespaces in exceptedNamespaces are not
// checked, but still report a passing case recording why.
func (n *noPodSecurityAdmissionLabelNamespaceChecker) generateTestCase(namespace corev1.Namespace) []*junitapi.JUnitTestCase {
	namespaceName := namespace.Name

	testName := fmt.Sprintf("[sig-auth] namespace %s must set the pod security admission level labels", namespaceName)

	if reason, isException := exceptedNamespaces[namespaceName]; isException {
		return []*junitapi.JUnitTestCase{{
			Name:      testName,
			SystemOut: fmt.Sprintf("namespace %q is an exception to the pod security admission label check and was not checked: %s", namespaceName, reason),
		}}
	}

	psaLabelPrefix := "pod-security.kubernetes.io/"

	problems := []string{}
	for _, suffix := range []string{"enforce", "audit", "warn"} {
		key := psaLabelPrefix + suffix
		if _, ok := namespace.Labels[key]; !ok {
			problems = append(problems, fmt.Sprintf("%s is not set", key))
		}
	}

	if len(problems) == 0 {
		return []*junitapi.JUnitTestCase{{Name: testName}}
	}

	failure := fmt.Sprintf("namespace %q does not set the pod security admission level labels: %s", namespaceName, strings.Join(problems, ", "))
	// Reported as a flake while managed namespaces are being labelled ahead of the PSA label
	// syncer retirement. Many are still unlabelled, so failing hard would fail every job.
	// Remove the trailing success case to turn this into a hard failure.
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
		if ns.GenerateName != "" {
			continue
		}
		// We are only checking OpenShift managed namespaces.
		isManagedNamespace := ns.Name == "openshift" || strings.HasPrefix(ns.Name, "openshift-")
		if !isManagedNamespace {
			continue
		}
		junits = append(junits, n.generateTestCase(ns)...)
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
	n.cfgClient, err = configv1.NewForConfig(adminRESTConfig)
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
