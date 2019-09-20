package util

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	"github.com/onsi/ginkgo/types"
	"github.com/onsi/gomega"
	"k8s.io/klog"

	kapiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kclientset "k8s.io/client-go/kubernetes"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/testfiles"
	"k8s.io/kubernetes/test/e2e/generated"

	// this appears to inexplicably auto-register global flags.
	_ "k8s.io/kubernetes/test/e2e/storage/drivers"

	projectv1 "github.com/openshift/api/project/v1"
	securityv1client "github.com/openshift/client-go/security/clientset/versioned"
	"github.com/openshift/origin/pkg/version"
)

var (
	reportFileName string
	syntheticSuite string
	quiet          bool
)

var TestContext *e2e.TestContextType = &e2e.TestContext

func InitStandardFlags() {
	e2e.RegisterCommonFlags(flag.CommandLine)
	e2e.RegisterClusterFlags(flag.CommandLine)

	// replaced by a bare import above.
	//e2e.RegisterStorageFlags()
}

func InitTest(dryRun bool) {
	InitDefaultEnvironmentVariables()
	// interpret synthetic input in `--ginkgo.focus` and/or `--ginkgo.skip`
	ginkgo.BeforeEach(checkSyntheticInput)

	TestContext.DeleteNamespace = os.Getenv("DELETE_NAMESPACE") != "false"
	TestContext.VerifyServiceAccount = true
	testfiles.AddFileSource(testfiles.BindataFileSource{
		Asset:      generated.Asset,
		AssetNames: generated.AssetNames,
	})
	TestContext.KubectlPath = "kubectl"
	TestContext.KubeConfig = KubeConfigPath()
	os.Setenv("KUBECONFIG", TestContext.KubeConfig)

	// "debian" is used when not set. At least GlusterFS tests need "custom".
	// (There is no option for "rhel" or "centos".)
	TestContext.NodeOSDistro = "custom"
	TestContext.MasterOSDistro = "custom"

	// load and set the host variable for kubectl
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(&clientcmd.ClientConfigLoadingRules{ExplicitPath: TestContext.KubeConfig}, &clientcmd.ConfigOverrides{})
	cfg, err := clientConfig.ClientConfig()
	if err != nil && !dryRun { // we don't need the host when doing a dryrun
		FatalErr(err)
	}
	if cfg != nil {
		TestContext.Host = cfg.Host
	}

	reportFileName = os.Getenv("TEST_REPORT_FILE_NAME")
	if reportFileName == "" {
		reportFileName = "junit"
	}

	quiet = os.Getenv("TEST_OUTPUT_QUIET") == "true"

	// Ensure that Kube tests run privileged (like they do upstream)
	TestContext.CreateTestingNS = createTestingNS

	klog.V(2).Infof("Extended test version %s", version.Get().String())
}

func ExecuteTest(t ginkgo.GinkgoTestingT, suite string) {
	var r []ginkgo.Reporter

	if dir := os.Getenv("TEST_REPORT_DIR"); len(dir) > 0 {
		TestContext.ReportDir = dir
	}

	if TestContext.ReportDir != "" {
		if err := os.MkdirAll(TestContext.ReportDir, 0755); err != nil {
			klog.Errorf("Failed creating report directory: %v", err)
		}
		defer e2e.CoreDump(TestContext.ReportDir)
	}

	if config.GinkgoConfig.FocusString == "" && config.GinkgoConfig.SkipString == "" {
		config.GinkgoConfig.SkipString = "Skipped"
	}

	gomega.RegisterFailHandler(ginkgo.Fail)

	if TestContext.ReportDir != "" {
		r = append(r, reporters.NewJUnitReporter(path.Join(TestContext.ReportDir, fmt.Sprintf("%s_%02d.xml", reportFileName, config.GinkgoConfig.ParallelNode))))
	}

	AnnotateTestSuite()

	if quiet {
		r = append(r, NewSimpleReporter())
		ginkgo.RunSpecsWithCustomReporters(t, suite, r)
	} else {
		ginkgo.RunSpecsWithDefaultAndCustomReporters(t, suite, r)
	}
}

func AnnotateTestSuite() {
	testRenamer := newGinkgoTestRenamerFromGlobals(e2e.TestContext.Provider, getNetworkSkips())

	ginkgo.WalkTests(testRenamer.maybeRenameTest)
}

func getNetworkSkips() []string {
	out, err := e2e.KubectlCmd("get", "network.operator.openshift.io", "cluster", "--template", "{{.spec.defaultNetwork.type}}{{if .spec.defaultNetwork.openshiftSDNConfig}} {{.spec.defaultNetwork.type}}/{{.spec.defaultNetwork.openshiftSDNConfig.mode}}{{end}}").CombinedOutput()
	if err != nil {
		e2e.Logf("Could not get network operator configuration: not adding any plugin-specific skips")
		return nil
	}
	return strings.Split(string(out), " ")
}

func newGinkgoTestRenamerFromGlobals(provider string, networkSkips []string) *ginkgoTestRenamer {
	var allLabels []string
	matches := make(map[string]*regexp.Regexp)
	stringMatches := make(map[string][]string)
	excludes := make(map[string]*regexp.Regexp)

	for label, items := range testMaps {
		sort.Strings(items)
		allLabels = append(allLabels, label)
		var remain []string
		for _, item := range items {
			re := regexp.MustCompile(item)
			if p, ok := re.LiteralPrefix(); ok {
				stringMatches[label] = append(stringMatches[label], p)
			} else {
				remain = append(remain, item)
			}
		}
		if len(remain) > 0 {
			matches[label] = regexp.MustCompile(strings.Join(remain, `|`))
		}
	}
	for label, items := range labelExcludes {
		sort.Strings(items)
		excludes[label] = regexp.MustCompile(strings.Join(items, `|`))
	}
	sort.Strings(allLabels)

	if provider != "" {
		excludedTests = append(excludedTests, fmt.Sprintf(`\[Skipped:%s\]`, provider))
	}
	for _, network := range networkSkips {
		excludedTests = append(excludedTests, fmt.Sprintf(`\[Skipped:Network/%s\]`, network))
	}
	klog.Infof("openshift-tests excluded test regex is %q", strings.Join(excludedTests, `|`))
	excludedTestsFilter := regexp.MustCompile(strings.Join(excludedTests, `|`))

	return &ginkgoTestRenamer{
		allLabels:     allLabels,
		stringMatches: stringMatches,
		matches:       matches,
		excludes:      excludes,

		excludedTestsFilter: excludedTestsFilter,
	}
}

type ginkgoTestRenamer struct {
	allLabels     []string
	stringMatches map[string][]string
	matches       map[string]*regexp.Regexp
	excludes      map[string]*regexp.Regexp

	excludedTestsFilter *regexp.Regexp
}

func (r *ginkgoTestRenamer) maybeRenameTest(name string, node types.TestNode) {
	labels := ""
	for {
		count := 0
		for _, label := range r.allLabels {
			if strings.Contains(name, label) {
				continue
			}

			var hasLabel bool
			for _, segment := range r.stringMatches[label] {
				hasLabel = strings.Contains(name, segment)
				if hasLabel {
					break
				}
			}
			if !hasLabel {
				if re := r.matches[label]; re != nil {
					hasLabel = r.matches[label].MatchString(name)
				}
			}

			if hasLabel {
				// TODO: remove when we no longer need it
				if re, ok := r.excludes[label]; ok && re.MatchString(name) {
					continue
				}
				count++
				labels += " " + label
				name += " " + label
			}
		}
		if count == 0 {
			break
		}
	}

	if !r.excludedTestsFilter.MatchString(name) {
		isSerial := strings.Contains(name, "[Serial]")
		isConformance := strings.Contains(name, "[Conformance]")
		switch {
		case isSerial && isConformance:
			node.SetText(node.Text() + " [Suite:openshift/conformance/serial/minimal]")
		case isSerial:
			node.SetText(node.Text() + " [Suite:openshift/conformance/serial]")
		case isConformance:
			node.SetText(node.Text() + " [Suite:openshift/conformance/parallel/minimal]")
		default:
			node.SetText(node.Text() + " [Suite:openshift/conformance/parallel]")
		}
	}
	if strings.Contains(node.CodeLocation().FileName, "/origin/test/") && !strings.Contains(node.Text(), "[Suite:openshift") {
		node.SetText(node.Text() + " [Suite:openshift]")
	}
	if strings.Contains(node.CodeLocation().FileName, "/kubernetes/test/e2e/") {
		node.SetText(node.Text() + " [Suite:k8s]")
	}
	node.SetText(node.Text() + labels)
}

// ProwGCPSetup makes sure certain required env vars are available in the case
// that extended tests are invoked directly via calls to ginkgo/extended.test
func InitDefaultEnvironmentVariables() {
	if ad := os.Getenv("ARTIFACT_DIR"); len(strings.TrimSpace(ad)) == 0 {
		os.Setenv("ARTIFACT_DIR", filepath.Join(os.TempDir(), "artifacts"))
	}
}

// TODO: Use either explicit tags (k8s.io) or https://github.com/onsi/ginkgo/pull/228 to implement this.
// isPackage determines wether the test is in a package.  Ideally would be implemented in ginkgo.
func isPackage(pkg string) bool {
	return strings.Contains(ginkgo.CurrentGinkgoTestDescription().FileName, pkg)
}

// TODO: For both is*Test functions, use either explicit tags (k8s.io) or https://github.com/onsi/ginkgo/pull/228
func isOriginTest() bool {
	return isPackage("/origin/test/")
}

func isKubernetesE2ETest() bool {
	return isPackage("/kubernetes/test/e2e/")
}

func testNameContains(name string) bool {
	return strings.Contains(ginkgo.CurrentGinkgoTestDescription().FullTestText, name)
}

func isOriginUpgradeTest() bool {
	return isPackage("/origin/test/e2e/upgrade/")
}

func skipTestNamespaceCustomization() bool {
	return (isPackage("/kubernetes/test/e2e/namespace.go") && (testNameContains("should always delete fast") || testNameContains("should delete fast enough")))
}

// createTestingNS ensures that kubernetes e2e tests have their service accounts in the privileged and anyuid SCCs
func createTestingNS(baseName string, c kclientset.Interface, labels map[string]string) (*kapiv1.Namespace, error) {
	if !strings.HasPrefix(baseName, "e2e-") {
		baseName = "e2e-" + baseName
	}
	ns, err := e2e.CreateTestingNS(baseName, c, labels)
	if err != nil {
		return ns, err
	}

	klog.V(2).Infof("blah=%s", ginkgo.CurrentGinkgoTestDescription().FileName)

	// Add anyuid and privileged permissions for upstream tests
	if (isKubernetesE2ETest() && !skipTestNamespaceCustomization()) || isOriginUpgradeTest() {
		clientConfig, err := getClientConfig(KubeConfigPath())
		if err != nil {
			return ns, err
		}
		securityClient, err := securityv1client.NewForConfig(clientConfig)
		if err != nil {
			return ns, err
		}
		e2e.Logf("About to run a Kube e2e test, ensuring namespace is privileged")
		// add the "privileged" scc to ensure pods that explicitly
		// request extra capabilities are not rejected
		addE2EServiceAccountsToSCC(securityClient, []kapiv1.Namespace{*ns}, "privileged")
		// add the "anyuid" scc to ensure pods that don't specify a
		// uid don't get forced into a range (mimics upstream
		// behavior)
		addE2EServiceAccountsToSCC(securityClient, []kapiv1.Namespace{*ns}, "anyuid")
		// add the "hostmount-anyuid" scc to ensure pods using hostPath
		// can execute tests
		addE2EServiceAccountsToSCC(securityClient, []kapiv1.Namespace{*ns}, "hostmount-anyuid")

		// The intra-pod test requires that the service account have
		// permission to retrieve service endpoints.
		rbacClient, err := rbacv1client.NewForConfig(clientConfig)
		if err != nil {
			return ns, err
		}
		addRoleToE2EServiceAccounts(rbacClient, []kapiv1.Namespace{*ns}, "view")

		// in practice too many kube tests ignore scheduling constraints
		allowAllNodeScheduling(c, ns.Name)
	}

	return ns, err
}

var (
	testMaps = map[string][]string{
		// tests that require a local host
		"[Local]": {
			// Doesn't work on scaled up clusters
			`\[Feature:ImagePrune\]`,
		},
		// alpha features that are not gated
		"[Disabled:Alpha]": {
			`\[Feature:Initializers\]`,     // admission controller disabled
			`\[Feature:TTLAfterFinished\]`, // flag gate is off
			`\[Feature:GPUDevicePlugin\]`,  // GPU node needs to be available
			`\[Feature:ExpandCSIVolumes\]`, // off by default .  sig-storage
			`\[Feature:DynamicAudit\]`,     // off by default.  sig-master

			`\[NodeAlphaFeature:VolumeSubpathEnvExpansion\]`, // flag gate is off
			`\[Feature:IPv6DualStack.*\]`,
		},
		// tests for features that are not implemented in openshift
		"[Disabled:Unimplemented]": {
			`\[Feature:Networking-IPv6\]`,     // openshift-sdn doesn't support yet
			`Monitoring`,                      // Not installed, should be
			`Cluster level logging`,           // Not installed yet
			`Kibana`,                          // Not installed
			`Ubernetes`,                       // Can't set zone labels today
			`kube-ui`,                         // Not installed by default
			`Kubernetes Dashboard`,            // Not installed by default (also probably slow image pull)
			`\[Feature:ServiceLoadBalancer\]`, // Not enabled yet
			`\[Feature:RuntimeClass\]`,        // disable runtimeclass tests in 4.1 (sig-pod/sjenning@redhat.com)

			`NetworkPolicy.*egress`,     // not supported
			`NetworkPolicy.*named port`, // not yet implemented
			`enforce egress policy`,     // not support
			`should proxy to cadvisor`,  // we don't expose cAdvisor port directly for security reasons
		},
		// tests that rely on special configuration that we do not yet support
		"[Disabled:SpecialConfig]": {
			`\[Feature:ImageQuota\]`,                    // Quota isn't turned on by default, we should do that and then reenable these tests
			`\[Feature:Audit\]`,                         // Needs special configuration
			`\[Feature:LocalStorageCapacityIsolation\]`, // relies on a separate daemonset?
			`\[sig-cluster-lifecycle\]`,                 // cluster lifecycle test require a different kind of upgrade hook.
			`\[Feature:StatefulUpgrade\]`,               // related to cluster lifecycle (in e2e/lifecycle package) and requires an upgrade hook we don't use

			`kube-dns-autoscaler`, // Don't run kube-dns
			`should check if Kubernetes master services is included in cluster-info`, // Don't run kube-dns
			`DNS configMap`, // this tests dns federation configuration via configmap, which we don't support yet

			`authentication: OpenLDAP`,              // needs separate setup and bucketing for openldap bootstrapping
			`NodeProblemDetector`,                   // requires a non-master node to run on
			`Advanced Audit should audit API calls`, // expects to be able to call /logs

			`Firewall rule should have correct firewall rules for e2e cluster`, // Upstream-install specific
		},
		// tests that are known broken and need to be fixed upstream or in openshift
		// always add an issue here
		"[Disabled:Broken]": {
			`mount an API token into pods`,                                               // We add 6 secrets, not 1
			`ServiceAccounts should ensure a single API token exists`,                    // We create lots of secrets
			`unchanging, static URL paths for kubernetes api services`,                   // the test needs to exclude URLs that are not part of conformance (/logs)
			`Simple pod should handle in-cluster config`,                                 // kubectl cp is not preserving executable bit
			`Services should be able to up and down services`,                            // we don't have wget installed on nodes
			`Network should set TCP CLOSE_WAIT timeout`,                                  // possibly some difference between ubuntu and fedora
			`Services should be able to create a functioning NodePort service`,           // https://bugzilla.redhat.com/show_bug.cgi?id=1711603
			`\[NodeFeature:Sysctls\]`,                                                    // needs SCC support
			`should check kube-proxy urls`,                                               // previously this test was skipped b/c we reported -1 as the number of nodes, now we report proper number and test fails
			`SSH`,                                                                        // TRIAGE
			`should implement service.kubernetes.io/service-proxy-name`,                  // this is an optional test that requires SSH. sig-network
			`should idle the service and DeploymentConfig properly`,                      // idling with a single service and DeploymentConfig [Conformance]
			`\[Driver: csi-hostpath`,                                                     // https://bugzilla.redhat.com/show_bug.cgi?id=1711607
			`should answer endpoint and wildcard queries for the cluster`,                // currently not supported by dns operator https://github.com/openshift/cluster-dns-operator/issues/43
			`should propagate mounts to the host`,                                        // requires SSH, https://bugzilla.redhat.com/show_bug.cgi?id=1711600
			`should allow ingress access on one named port`,                              // https://bugzilla.redhat.com/show_bug.cgi?id=1711602
			`ClusterDns \[Feature:Example\] should create pod that uses dns`,             // https://bugzilla.redhat.com/show_bug.cgi?id=1711601
			`should be rejected when no endpoints exist`,                                 // https://bugzilla.redhat.com/show_bug.cgi?id=1711605
			`PreemptionExecutionPath runs ReplicaSets to verify preemption running path`, // https://bugzilla.redhat.com/show_bug.cgi?id=1711606
			`TaintBasedEvictions`,                                                        // https://bugzilla.redhat.com/show_bug.cgi?id=1711608
			// TODO(workloads): reenable
			`SchedulerPreemption`,

			`\[Driver: iscsi\]`, // https://bugzilla.redhat.com/show_bug.cgi?id=1711627

			`\[Driver: nfs\] \[Testpattern: Dynamic PV \(default fs\)\] provisioning should access volume from different nodes`, // https://bugzilla.redhat.com/show_bug.cgi?id=1711688

			// Test fails on platforms that use LoadBalancerService and HostNetwork endpoint publishing strategy
			`\[Conformance\]\[Area:Networking\]\[Feature:Router\] The HAProxy router should set Forwarded headers appropriately`, // https://bugzilla.redhat.com/show_bug.cgi?id=1752646

			// requires a 1.14 kubelet, enable when rhcos is built for 4.2
			"when the NodeLease feature is enabled",
			"RuntimeClass should reject",

			// TODO(node): configure the cri handler for the runtime class to make this work
			"should run a Pod requesting a RuntimeClass with a configured handler",
			"should reject a Pod requesting a RuntimeClass with conflicting node selector",
			"should run a Pod requesting a RuntimeClass with scheduling",

			// TODO(sdn): reenable when openshift/sdn is rebased to 1.16
			`Services should implement service.kubernetes.io/headless`,

			// TODO(sdn): test pod fails to connect in 1.16
			`should allow ingress access from updated pod`,

			// TODO(storage): fix the use of SSH into the node
			`volumeMode should not mount / map unused volumes in a pod`,

			// TODO(workload): reactivate when oc is rebased
			`should support exec using resource/name`,
		},
		// tests that may work, but we don't support them
		"[Disabled:Unsupported]": {
			`\[Driver: rbd\]`,  // OpenShift 4.x does not support Ceph RBD (use CSI instead)
			`\[Driver: ceph\]`, // OpenShift 4.x does not support CephFS (use CSI instead)
		},
		// tests too slow to be part of conformance
		"[Slow]": {
			`\[sig-scalability\]`,                          // disable from the default set for now
			`should create and stop a working application`, // Inordinately slow tests

			`\[Feature:PerformanceDNS\]`, // very slow

			`should ensure that critical pod is scheduled in case there is no resources available`, // should be tagged disruptive, consumes 100% of cluster CPU

			`validates that there exists conflict between pods with same hostPort and protocol but one using 0\.0\.0\.0 hostIP`, // 5m, really?
		},
		// tests that are known flaky
		"[Flaky]": {
			`Job should run a job to completion when tasks sometimes fail and are not locally restarted`, // seems flaky, also may require too many resources
			`openshift mongodb replication creating from a template`,                                     // flaking on deployment

			// TODO(node): test works when run alone, but not in the suite in CI
			`\[Feature:HPA\] Horizontal pod autoscaling \(scale resource: CPU\) \[sig-autoscaling\] ReplicationController light Should scale from 1 pod to 2 pods`,
		},
		// tests that must be run without competition
		"[Serial]": {
			`\[Disruptive\]`,
			`\[Feature:Performance\]`,            // requires isolation
			`\[Feature:ManualPerformance\]`,      // requires isolation
			`\[Feature:HighDensityPerformance\]`, // requires no other namespaces

			`Service endpoints latency`, // requires low latency
			`Clean up pods on node`,     // schedules up to max pods per node
			`should allow starting 95 pods per node`,
			`DynamicProvisioner should test that deleting a claim before the volume is provisioned deletes the volume`, // test is very disruptive to other tests

			`Should be able to support the 1\.7 Sample API Server using the current Aggregator`, // down apiservices break other clients today https://bugzilla.redhat.com/show_bug.cgi?id=1623195

			`\[Feature:HPA\] Horizontal pod autoscaling \(scale resource: CPU\) \[sig-autoscaling\] ReplicationController light Should scale from 1 pod to 2 pods`,
		},
		"[Skipped:azure]": {
			"Networking should provide Internet connection for containers", // Azure does not allow ICMP traffic to internet.

			// Azure storage tests are failing due to unknown errors. ref: https://bugzilla.redhat.com/show_bug.cgi?id=1723603
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Dynamic PV \(default fs\)\] provisioning should access volume from different nodes`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Dynamic PV \(default fs\)\] subPath should verify container cannot write to subpath readonly volumes`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Inline-volume \(default fs\)\] subPath should be able to unmount after the subpath directory is deleted`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Inline-volume \(default fs\)\] subPath should support existing directories when readOnly specified in the volumeSource`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Inline-volume \(default fs\)\] subPath should support existing directory`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Inline-volume \(default fs\)\] subPath should support existing single file`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Inline-volume \(default fs\)\] subPath should support file as subpath`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Inline-volume \(default fs\)\] subPath should support non-existent path`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Inline-volume \(default fs\)\] subPath should support readOnly directory specified in the volumeMount`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Inline-volume \(default fs\)\] subPath should support readOnly file specified in the volumeMount`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Inline-volume \(default fs\)\] subPath should verify container cannot write to subpath readonly volumes`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Inline-volume \(default fs\)\] volumes should allow exec of files on the volume`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Inline-volume \(default fs\)\] volumes should be mountable`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Inline-volume \(ext4\)\] volumes should allow exec of files on the volume`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Inline-volume \(ext4\)\] volumes should be mountable`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Pre-provisioned PV \(block volmode\)] volumeMode should create sc, pod, pv, and pvc, read/write to the pv, and delete all created resources`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Pre-provisioned PV \(default fs\)\] subPath should be able to unmount after the subpath directory is deleted`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Pre-provisioned PV \(default fs\)\] subPath should support existing directories when readOnly specified in the volumeSource`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Pre-provisioned PV \(default fs\)\] subPath should support existing directory`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Pre-provisioned PV \(default fs\)\] subPath should support existing single file`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Pre-provisioned PV \(default fs\)\] subPath should support file as subpath`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Pre-provisioned PV \(default fs\)\] subPath should support non-existent path`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Pre-provisioned PV \(default fs\)\] subPath should support readOnly directory specified in the volumeMount`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Pre-provisioned PV \(default fs\)\] subPath should support readOnly file specified in the volumeMount`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Pre-provisioned PV \(default fs\)\] subPath should verify container cannot write to subpath readonly volumes`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Pre-provisioned PV \(default fs\)\] volumes should allow exec of files on the volume`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Pre-provisioned PV \(default fs\)\] volumes should be mountable`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Pre-provisioned PV \(ext4\)\] volumes should allow exec of files on the volume`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Pre-provisioned PV \(ext4\)\] volumes should be mountable`,
			`\[sig-storage\] In-tree Volumes \[Driver: azure\] \[Testpattern: Pre-provisioned PV \(filesystem volmode\)] volumeMode should create sc, pod, pv, and pvc, read/write to the pv, and delete all created resources`,
		},
		"[Skipped:gce]": {
			// Requires creation of a different compute instance in a different zone and is not compatible with volumeBindingMode of WaitForFirstConsumer which we use in 4.x
			`\[sig-scheduling\] Multi-AZ Cluster Volumes \[sig-storage\] should only be allowed to provision PDs in zones where nodes exist`,

			// The following tests try to ssh directly to a node. None of our nodes have external IPs
			`\[k8s.io\] \[sig-node\] crictl should be able to run crictl on the node`,
			`\[sig-storage\] Flexvolumes should be mountable`,
			`\[sig-storage\] Detaching volumes should not work when mount is in progress`,

			// We are using openshift-sdn to conceal metadata
			`\[sig-auth\] Metadata Concealment should run a check-metadata-concealment job to completion`,

			// https://bugzilla.redhat.com/show_bug.cgi?id=1740959
			`\[sig-api-machinery\] AdmissionWebhook Should be able to deny pod and configmap creation`,

			// https://bugzilla.redhat.com/show_bug.cgi?id=1745720
			`\[sig-storage\] CSI Volumes \[Driver: pd.csi.storage.gke.io\]\[Serial\]`,

			// https://bugzilla.redhat.com/show_bug.cgi?id=1749882
			`\[sig-storage\] CSI Volumes CSI Topology test using GCE PD driver \[Serial\]`,

			// https://bugzilla.redhat.com/show_bug.cgi?id=1751367
			`gce-localssd-scsi-fs`,

			// https://bugzilla.redhat.com/show_bug.cgi?id=1750851
			// should be serial if/when it's re-enabled
			`\[HPA\] Horizontal pod autoscaling \(scale resource: Custom Metrics from Stackdriver\)`,
		},
		// tests that don't pass under openshift-sdn but that are expected to pass
		// with other network plugins (particularly ovn-kubernetes)
		"[Skipped:Network/OpenShiftSDN]": {
			`NetworkPolicy between server and client should allow egress access on one named port`, // not yet implemented
		},
		// tests that don't pass under openshift-sdn multitenant mode
		"[Skipped:Network/OpenShiftSDN/Multitenant]": {
			`\[Feature:NetworkPolicy\]`, // not compatible with multitenant mode
			`\[sig-network\] Services should preserve source pod IP for traffic thru service cluster IP`, // known bug, not planned to be fixed
		},
		// tests that don't pass under OVN Kubernetes
		"[Skipped:Network/OVNKubernetes]": {
			`\[sig-network\] Services should be able to switch session affinity for NodePort service`,            // https://jira.coreos.com/browse/SDN-510
			`\[sig-network\] Services should be able to switch session affinity for service with type clusterIP`, // https://jira.coreos.com/browse/SDN-510
			`\[sig-network\] Services should have session affinity work for NodePort service`,                    // https://jira.coreos.com/browse/SDN-510
			`\[sig-network\] Services should have session affinity work for service with type clusterIP`,         // https://jira.coreos.com/browse/SDN-510
		},
		"[Suite:openshift/scalability]": {},
		// tests that replace the old test-cmd script
		"[Suite:openshift/test-cmd]": {
			`\[Suite:openshift/test-cmd\]`,
		},
		"[Suite:openshift/csi]": {
			`External Storage \[Driver:`,
		},
	}

	// labelExcludes temporarily block tests out of a specific suite
	labelExcludes = map[string][]string{}

	excludedTests = []string{
		`\[Disabled:`,
		`\[Disruptive\]`,
		`\[Skipped\]`,
		`\[Slow\]`,
		`\[Flaky\]`,
		`\[local\]`,
		`\[Suite:openshift/test-cmd\]`,
	}
)

// checkSyntheticInput selects tests based on synthetic skips or focuses
func checkSyntheticInput() {
	checkSuiteSkips()
}

// checkSuiteSkips ensures Origin/Kubernetes synthetic skip labels are applied
// DEPRECATED: remove in a future release
func checkSuiteSkips() {
	switch {
	case isOriginTest():
		if strings.Contains(config.GinkgoConfig.SkipString, "Synthetic Origin") {
			ginkgo.Skip("skipping all openshift/origin tests")
		}
	case isKubernetesE2ETest():
		if strings.Contains(config.GinkgoConfig.SkipString, "Synthetic Kubernetes") {
			ginkgo.Skip("skipping all k8s.io/kubernetes tests")
		}
	}
}

var longRetry = wait.Backoff{Steps: 100}

// allowAllNodeScheduling sets the annotation on namespace that allows all nodes to be scheduled onto.
func allowAllNodeScheduling(c kclientset.Interface, namespace string) {
	err := retry.RetryOnConflict(longRetry, func() error {
		ns, err := c.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if ns.Annotations == nil {
			ns.Annotations = make(map[string]string)
		}
		ns.Annotations[projectv1.ProjectNodeSelector] = ""
		_, err = c.CoreV1().Namespaces().Update(ns)
		return err
	})
	if err != nil {
		FatalErr(err)
	}
}

func addE2EServiceAccountsToSCC(securityClient securityv1client.Interface, namespaces []kapiv1.Namespace, sccName string) {
	// Because updates can race, we need to set the backoff retries to be > than the number of possible
	// parallel jobs starting at once. Set very high to allow future high parallelism.
	err := retry.RetryOnConflict(longRetry, func() error {
		scc, err := securityClient.SecurityV1().SecurityContextConstraints().Get(sccName, metav1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				return nil
			}
			return err
		}

		for _, ns := range namespaces {
			if isE2ENamespace(ns.Name) {
				scc.Groups = append(scc.Groups, fmt.Sprintf("system:serviceaccounts:%s", ns.Name))
			}
		}
		if _, err := securityClient.SecurityV1().SecurityContextConstraints().Update(scc); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		FatalErr(err)
	}
}

func isE2ENamespace(ns string) bool {
	return true
	//return strings.HasPrefix(ns, "e2e-") ||
	//	strings.HasPrefix(ns, "aggregator-") ||
	//	strings.HasPrefix(ns, "csi-") ||
	//	strings.HasPrefix(ns, "deployment-") ||
	//	strings.HasPrefix(ns, "disruption-") ||
	//	strings.HasPrefix(ns, "gc-") ||
	//	strings.HasPrefix(ns, "kubectl-") ||
	//	strings.HasPrefix(ns, "proxy-") ||
	//	strings.HasPrefix(ns, "provisioning-") ||
	//	strings.HasPrefix(ns, "statefulset-") ||
	//	strings.HasPrefix(ns, "services-")
}

func addRoleToE2EServiceAccounts(rbacClient rbacv1client.RbacV1Interface, namespaces []kapiv1.Namespace, roleName string) {
	err := retry.RetryOnConflict(longRetry, func() error {
		for _, ns := range namespaces {
			if isE2ENamespace(ns.Name) && ns.Status.Phase != kapiv1.NamespaceTerminating {
				_, err := rbacClient.RoleBindings(ns.Name).Create(&rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{GenerateName: "default-" + roleName, Namespace: ns.Name},
					RoleRef: rbacv1.RoleRef{
						Kind: "ClusterRole",
						Name: roleName,
					},
					Subjects: []rbacv1.Subject{
						{Name: "default", Namespace: ns.Name, Kind: rbacv1.ServiceAccountKind},
					},
				})
				if err != nil {
					e2e.Logf("Warning: Failed to add role to e2e service account: %v", err)
				}
			}
		}
		return nil
	})
	if err != nil {
		FatalErr(err)
	}
}
