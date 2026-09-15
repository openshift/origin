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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type noPodSecurityAdmissionLabelNamespaceChecker struct {
	kubeClient kubernetes.Interface
	cfgClient  *configv1.ConfigV1Client
}

// Cleanup implements monitortestframework.MonitorTest
func (n *noPodSecurityAdmissionLabelNamespaceChecker) Cleanup(ctx context.Context) error {
	return nil
}

// generateTestCases evaluates that namespaces are using the label
// security.openshift.io/scc.podSecurityLabelSync=false
// It returns the evaluated test cases or an error if any errors are encountered during the namespace evaluation.
func (n *noPodSecurityAdmissionLabelNamespaceChecker) generateTestCase(ctx context.Context, namespaceName string) (*junitapi.JUnitTestCase, error) {
	namespace, err := n.kubeClient.CoreV1().Namespaces().Get(ctx, namespaceName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	testName := fmt.Sprintf("[sig-auth] namespace %s must be labelled correctly by PSA Label Syncer", namespaceName)
	junitObject := &junitapi.JUnitTestCase{Name: testName}

	syncLabelValue, hasSyncLabel := namespace.Labels["security.openshift.io/scc.podSecurityLabelSync"]
	psaLabelPrefix := "pod-security.kubernetes.io/"

	numberOfPSALabels := 0
	for key := range namespace.Labels {
		if strings.HasPrefix(key, psaLabelPrefix) {
			numberOfPSALabels++
		}
	}

	hasAllPSALabels := numberOfPSALabels >= 6
	isOptedOut := hasSyncLabel && syncLabelValue == "false"
	isOptedInExplicitly := hasSyncLabel && syncLabelValue == "true"

	// Rule breakdown:
	// A namespace IS MANAGED by the syncer if:
	// 1. It is NOT explicitly opted out (sync != false) AND at least one PSA label is missing/not user-defined.
	// 2. OR it has sync=true (user explicitly asks syncer to manage all labels).
	isManaged := (!isOptedOut && !hasAllPSALabels) || isOptedInExplicitly

	if isManaged && numberOfPSALabels < 6 {
		// If it is managed by the syncer, the syncer MUST have populated all 6 PSA labels.
		failure := fmt.Sprintf("namespace %q is managed by PSA Label Syncer but is missing required PSA labels (found %d/6)", namespaceName, numberOfPSALabels)
		junitObject.SystemOut = failure
		junitObject.FailureOutput = &junitapi.FailureOutput{Output: failure}
	}

	return junitObject, nil
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
		// We are only checking non openshift, openshift-, kube- and default namespaces.
		if ns.Name == "default" || ns.Name == "openshift" || strings.HasPrefix(ns.Name, "openshift-") || strings.HasPrefix(ns.Name, "kube-") {
			continue
		}
		testCase, err := n.generateTestCase(ctx, ns.Name)
		if err != nil {
			return nil, nil, err
		}
		junits = append(junits, testCase)
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
