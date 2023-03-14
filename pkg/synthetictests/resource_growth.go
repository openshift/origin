package synthetictests

import (
	"context"
	"fmt"
	v1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/openshift/origin/test/e2e/upgrade"
	exutil "github.com/openshift/origin/test/extended/util"
	helper "github.com/openshift/origin/test/extended/util/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	// allowedResourceGrowth is the multiplier we'll allow before failing the test. (currently 40%)
	allowedResourceGrowth = 1.4
	ovnNamespace          = "openshift-ovn-kubernetes"
)

// dont dump info more than 1 time per run
var debugOVNExecuted bool

func testNoExcessiveSecretGrowthDuringUpgrade() []*junitapi.JUnitTestCase {
	const testName = "[sig-trt] Secret count should not have grown significantly during upgrade"
	tests := comparePostUpgradeResourceCountFromMetrics(testName, "secretes")
	for _, test := range tests {
		if test.FailureOutput != nil && !debugOVNExecuted {
			debugOVN()
			debugOVNExecuted = true
		}
	}
	return tests

}

func testNoExcessiveConfigMapGrowthDuringUpgrade() []*junitapi.JUnitTestCase {
	const testName = "[sig-trt] ConfigMap count should not have grown significantly during upgrade"
	tests := comparePostUpgradeResourceCountFromMetrics(testName, "configmaps")
	for _, test := range tests {
		if test.FailureOutput != nil && !debugOVNExecuted {
			debugOVN()
			debugOVNExecuted = true
		}
	}
	return tests
}

// comparePostUpgradeResourceCountFromMetrics tests that some counts for certain resources we're most interested
// in potentially leaking do not increase substantially during upgrade.
// This is in response to a bug discovered where operators were leaking Secrets and ultimately taking down clusters.
// The counts have to be recorded before upgrade which is done in test/e2e/upgrade/upgrade.go, stored in a package
// variable, and then read here in invariant. This is to work around the problems where our normal ginko tests are
// all run in separate processes by themselves.
//
// Values for comparison are a guess at what would have caught the leak we saw. (growth of about 60% during upgrade)
//
// resource should be all lowercase and plural such as "secrets".
func comparePostUpgradeResourceCountFromMetrics(testName, resource string) []*junitapi.JUnitTestCase {
	oc := exutil.NewCLI("resource-growth-test")
	ctx := context.Background()

	preUpgradeCount := upgrade.PreUpgradeResourceCounts[resource]
	e2e.Logf("found %d %s prior to upgrade", preUpgradeCount, resource)

	// if the check on clusterversion returns a junit, we are done. most likely there was a problem
	// getting the cv or possibly the upgrade completion time was null because the rollback is still in
	// progress and the test has given up.
	cv, junit := getAndCheckClusterVersion(testName, oc)
	if junit != nil {
		return junit
	}
	upgradeCompletion := cv.Status.History[0].CompletionTime

	// Use prometheus to get the resource count at the moment we recorded upgrade complete. We can't do this
	// for the starting count as prometheus metrics seem to get wiped during the upgrade. We also don't want to
	// just list the resources right now, as we don't know what other tests might have created since.
	e2e.Logf("querying metrics at: %s", upgradeCompletion.Time.UTC().Format(time.RFC3339))
	resourceCountPromQuery := fmt.Sprintf(`cluster:usage:resources:sum{resource="%s"}`, resource)
	promResultsCompletion, err := helper.RunQueryAtTime(ctx, oc.NewPrometheusClient(ctx),
		resourceCountPromQuery, upgradeCompletion.Time)
	if err != nil {
		return []*junitapi.JUnitTestCase{
			{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: "Error getting resource count from Prometheus at upgrade completion time: " + err.Error(),
				},
			},
		}

	}
	e2e.Logf("got %d metrics after upgrade", len(promResultsCompletion.Data.Result))
	if len(promResultsCompletion.Data.Result) == 0 {
		return []*junitapi.JUnitTestCase{
			{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: "Post-upgrade resource count metric data has no Result",
				},
			},
		}
	}
	completedCount := int(promResultsCompletion.Data.Result[0].Value)
	e2e.Logf("found %d %s after upgrade", completedCount, resource)

	// Ensure that a resource count did not grow more than allowed:
	maxAllowedCount := int(float64(preUpgradeCount) * allowedResourceGrowth)
	output := fmt.Sprintf("%s count grew from %d to %d during upgrade (max allowed=%d). This test is experimental and may need adjusting in some cases.",
		resource, preUpgradeCount, completedCount, maxAllowedCount)

	if completedCount > maxAllowedCount {
		return []*junitapi.JUnitTestCase{
			{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: output,
				},
			},
		}
	}
	return []*junitapi.JUnitTestCase{
		{Name: testName},
	}

}

func getAndCheckClusterVersion(testName string, oc *exutil.CLI) (*v1.ClusterVersion, []*junitapi.JUnitTestCase) {
	cv, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(context.Background(), "version",
		metav1.GetOptions{})

	if err != nil {
		return cv, []*junitapi.JUnitTestCase{
			{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: "Error getting ClusterVersion: " + err.Error(),
				},
			},
		}

	}
	if len(cv.Status.History) == 0 {
		return cv, []*junitapi.JUnitTestCase{
			{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: "ClusterVersion.Status has no History",
				},
			},
		}
	}

	// In the case that cv completionTime is nil meaning the version change (upgrade or
	// rollback is still in progress), flake this case. The problem is likely a bigger
	// problem and the job will fail for that. Returning a failure and fake success, so it
	// will be marked as a flake.
	if cv.Status.History[0].CompletionTime == nil {
		return cv, []*junitapi.JUnitTestCase{
			{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: "ClusterVersion.completionTime is nil",
				},
			},
			{
				Name: testName,
			},
		}
	}

	return cv, nil
}

func debugOVN() {
	e2e.Logf("Dumping all relevant OVN information")
	oc := exutil.NewCLI(ovnNamespace)
	oc.SetNamespace(ovnNamespace)
	reachabilityCheck(oc)
	dumpMaster(oc)
	dumpNode(oc)
	ovnTrace()
	ovsTrace()

}

// if master is set, only get masters, otherwise get only ovnkube node pods
func getOVNPods(oc *exutil.CLI, master bool) ([]corev1.Pod, error) {
	pods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-ovn-kubernetes").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	key := "node"
	if master {
		key = "master"
	}
	var nodePods []corev1.Pod
	for _, pod := range pods.Items {
		if strings.Contains(pod.Name, key) {
			nodePods = append(nodePods, pod)
		}
	}

	return nodePods, nil
}

func executeCmdsOnPods(oc *exutil.CLI, commands [][]string, pods []corev1.Pod, master bool) {
	for _, pod := range pods {
		e2e.Logf("Dumping Information for node: %s, pod: %s", pod.Spec.NodeName, pod.Name)
		for _, cmd := range commands {
			rCmd := []string{pod.Name, "-c", "ovnkube-node", "--"}
			if master {
				rCmd = []string{pod.Name, "-c", "nbdb", "--"}
			}
			rCmd = append(rCmd, cmd...)
			output, err := oc.AsAdmin().Run("exec").Args(rCmd...).Output()
			e2e.Logf("Node Output from command: %#v, error: %s, output: %s", cmd, err, output)
		}
	}
}

// get every node and dump all flows, groups, interfaces, conntrack, etc
func dumpNode(oc *exutil.CLI) {
	e2e.Logf("Dumping OVN node information...")
	pods, err := getOVNPods(oc, false)
	if err != nil {
		e2e.Logf("Failed to execute dump node: %v", err)
		return
	}

	commands := [][]string{
		{"ovs-ofctl", "dump-flows", "br-int"},
		{"ovs-ofctl", "dump-groups", "br-int"},
		{"ovs-vsctl", "list", "interface"},
		{"ovs-ofctl", "show", "br-int"},
		{"ovs-vsctl", "show"},
		{"ovs-ofctl", "dump-flows", "br-ex"},
		{"ovs-ofctl", "show", "br-ex"},
		{"conntrack", "-L"},
	}

	executeCmdsOnPods(oc, commands, pods, false)

}

// dump OVN information
func dumpMaster(oc *exutil.CLI) {
	e2e.Logf("Dumping OVN master information...")
	pods, err := getOVNPods(oc, true)
	if err != nil {
		e2e.Logf("Failed to execute dump master: %v", err)
		return
	}

	commands := [][]string{
		{"ovn-nbctl", "--no-leader-only", "show"},
		{"ovn-nbctl", "--no-leader-only", "list", "load_balancer"},
		{"ovn-nbctl", "--no-leader-only", "list", "logical_switch"},
		{"ovn-nbctl", "--no-leader-only", "list", "logical_router"},
		{"ovn-sbctl", "--no-leader-only", "lflow-list"},
	}

	executeCmdsOnPods(oc, commands, pods, true)
}

func reachabilityCheck(oc *exutil.CLI) {
	e2e.Logf("Reachability check running...")
	pods, err := getOVNPods(oc, false)
	if err != nil {
		e2e.Logf("Failed to get pods for reachability check: %v", err)
		return
	}

	// get a kapi endpoint and curl from each worker node
	eps, err := oc.AdminKubeClient().DiscoveryV1().EndpointSlices("default").Get(context.TODO(), "kubernetes", metav1.GetOptions{})
	if err != nil {
		e2e.Logf("Failed to get endpoint slices for kubernetes service: %v", err)
	}

	// get first endpoint
	var endpoint string
	for _, ep := range eps.Endpoints {
		if len(ep.Addresses) > 0 {
			endpoint = ep.Addresses[0]
			break
		}
	}

	if len(endpoint) == 0 {
		e2e.Logf("Failed to find endpoint for kubernetes service: %#v", eps.Endpoints)
		return
	}

	var k8sSvcIP string
	svc, err := oc.AdminKubeClient().CoreV1().Services("default").Get(context.TODO(), "kubernetes", metav1.GetOptions{})
	if err != nil {
		e2e.Logf("Failed to find service for kubernetes")
	} else {
		k8sSvcIP = svc.Spec.ClusterIP
	}

	reachabilityEndpointStatus := "PASS"
	reachabilityServiceStatus := "PASS"
	// curl the endpoint directly, then if that works try the service
	for _, pod := range pods {
		output, err := oc.AsAdmin().Run("exec").Args(pod.Name, "-c", "ovnkube-node", "--", "curl",
			"--max-time", "2", "-k", fmt.Sprintf("https://%s:6443", endpoint)).Output()
		if err != nil || !strings.Contains(output, "Forbidden") {
			e2e.Logf("Reachability check failed to endpoint on pod/node %s/%s. Error: %v, Output: %s",
				pod.Name, pod.Spec.NodeName, err, output)
			reachabilityEndpointStatus = "FAIL"
			// skip checking service if we cant get to endpoint
			reachabilityServiceStatus = "FAIL"
			continue
		}

		// curl service
		if len(k8sSvcIP) > 0 {
			output, err := oc.AsAdmin().Run("exec").Args(pod.Name, "-c", "ovnkube-node", "--", "curl",
				"--max-time", "2", "-k", fmt.Sprintf("https://%s:443", k8sSvcIP)).Output()
			if err != nil || !strings.Contains(output, "Forbidden") {
				e2e.Logf("Reachability check failed to service on pod/node %s/%s. Error: %v, Output: %s",
					pod.Name, pod.Spec.NodeName, err, output)
				reachabilityServiceStatus = "FAIL"
			}
		}
	}

	e2e.Logf("Reachability check completed with endpoint/service status: %s/%s", reachabilityEndpointStatus, reachabilityServiceStatus)
}

// trace ovs access to kapi service
func ovsTrace() {
	// TODO
	return
}

// ovn-trace to kapi service from pod
func ovnTrace() {
	// TODO
	return
}
