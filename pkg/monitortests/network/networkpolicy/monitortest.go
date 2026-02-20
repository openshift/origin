package networkpolicy

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type networkPolicyMonitorTest struct {
	kubeClient kubernetes.Interface
}

// NewNetworkPolicyMonitorTest creates a monitor test that requires that every core platform namespace
// has a default deny-all Ingress/Egress NetworkPolicy defined. See https://issues.redhat.com/browse/OCPSTRAT-1046
// for more details.
func NewNetworkPolicyMonitorTest() monitortestframework.MonitorTest {
	return &networkPolicyMonitorTest{}
}

func (n *networkPolicyMonitorTest) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (n *networkPolicyMonitorTest) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	var err error
	n.kubeClient, err = kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	return nil
}

func (n *networkPolicyMonitorTest) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if n.kubeClient == nil {
		return nil, nil, nil
	}

	nsList, err := n.kubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("could not list all namespaces; %v", err)
	}

	junits := make([]*junitapi.JUnitTestCase, 0)
	for _, ns := range nsList.Items {

		// skip namespaces that aren't core openshift
		if ns.Name != "openshift" && !strings.HasPrefix(ns.Name, "openshift-") {
			continue
		}

		// skip managed service namespaces
		if exutil.ManagedServiceNamespaces.Has(ns.Name) {
			continue
		}

		testCase := &junitapi.JUnitTestCase{
			Name: fmt.Sprintf("[sig-network] ns/%s must have a default deny-all ingress/egress NetworkPolicy", ns.Name),
		}

		networkPolicies, err := n.kubeClient.NetworkingV1().NetworkPolicies(ns.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("could not list network policies of ns/%s", ns.Name)
		}

		// check if namespace contains a deny-all default network policy
		if !slices.ContainsFunc(networkPolicies.Items, isDenyAllPolicy) {
			failureMsg := "no default deny-all NetworkPolicy found in namespace"
			testCase.SystemOut = failureMsg
			testCase.FailureOutput = &junitapi.FailureOutput{Output: failureMsg}
		}

		junits = append(junits, testCase)
	}

	return nil, junits, nil
}

func (n *networkPolicyMonitorTest) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (n *networkPolicyMonitorTest) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	junits := []*junitapi.JUnitTestCase{}
	return junits, nil
}

func (n *networkPolicyMonitorTest) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (n *networkPolicyMonitorTest) Cleanup(ctx context.Context) error {
	return nil
}

// isDenyAllPolicy checks if a NetworkPolicy is a deny-all policy by validating:
// 1. PodSelector is empty (selects all pods in namespace)
// 2. PolicyTypes includes both Ingress and Egress
// 3. Ingress rules are empty (denies all ingress)
// 4. Egress rules are empty (denies all egress)
func isDenyAllPolicy(policy networkingv1.NetworkPolicy) bool {
	// PodSelector must be empty (selects all pods)
	if len(policy.Spec.PodSelector.MatchLabels) > 0 {
		return false
	}

	if len(policy.Spec.PodSelector.MatchExpressions) > 0 {
		return false
	}

	// policy must specify exactly both Ingress and Egress types
	if len(policy.Spec.PolicyTypes) != 2 ||
		!slices.Contains(policy.Spec.PolicyTypes, networkingv1.PolicyTypeIngress) ||
		!slices.Contains(policy.Spec.PolicyTypes, networkingv1.PolicyTypeEgress) {
		return false
	}

	// ingress rules must be empty (deny all ingress)
	if len(policy.Spec.Ingress) > 0 {
		return false
	}

	// egress rules must be empty (deny all egress)
	if len(policy.Spec.Egress) > 0 {
		return false
	}

	return true
}
