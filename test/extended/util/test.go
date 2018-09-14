package util

import (
	"flag"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/golang/glog"
	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	"github.com/onsi/ginkgo/types"
	"github.com/onsi/gomega"

	kapiv1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kclientset "k8s.io/client-go/kubernetes"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/oc/cli/admin/policy"
	securityclient "github.com/openshift/origin/pkg/security/generated/internalclientset"
	"github.com/openshift/origin/pkg/version"
	testutil "github.com/openshift/origin/test/util"
)

var (
	reportFileName string
	syntheticSuite string
	quiet          bool
)

var TestContext *e2e.TestContextType = &e2e.TestContext

// init initialize the extended testing suite.
// You can set these environment variables to configure extended tests:
// KUBECONFIG - Path to kubeconfig containing embedded authinfo
// TEST_REPORT_DIR - If set, JUnit output will be written to this directory for each test
// TEST_REPORT_FILE_NAME - If set, will determine the name of the file that JUnit output is written to
func InitTest() {
	// interpret synthetic input in `--ginkgo.focus` and/or `--ginkgo.skip`
	ginkgo.BeforeEach(checkSyntheticInput)

	flag.StringVar(&syntheticSuite, "suite", "", "DEPRECATED: Optional suite selector to filter which tests are run. Use focus.")
	e2e.ViperizeFlags()

	TestContext.DeleteNamespace = os.Getenv("DELETE_NAMESPACE") != "false"
	TestContext.VerifyServiceAccount = true
	TestContext.RepoRoot = os.Getenv("KUBE_REPO_ROOT")
	TestContext.KubeVolumeDir = os.Getenv("VOLUME_DIR")
	if len(TestContext.KubeVolumeDir) == 0 {
		TestContext.KubeVolumeDir = "/var/lib/origin/volumes"
	}
	TestContext.KubectlPath = "kubectl"
	TestContext.KubeConfig = KubeConfigPath()
	os.Setenv("KUBECONFIG", TestContext.KubeConfig)

	// load and set the host variable for kubectl
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(&clientcmd.ClientConfigLoadingRules{ExplicitPath: TestContext.KubeConfig}, &clientcmd.ConfigOverrides{})
	cfg, err := clientConfig.ClientConfig()
	if err != nil {
		FatalErr(err)
	}
	TestContext.Host = cfg.Host

	reportFileName = os.Getenv("TEST_REPORT_FILE_NAME")
	if reportFileName == "" {
		reportFileName = "junit"
	}

	quiet = os.Getenv("TEST_OUTPUT_QUIET") == "true"

	// Ensure that Kube tests run privileged (like they do upstream)
	TestContext.CreateTestingNS = createTestingNS

	glog.Infof("Extended test version %s", version.Get().String())
}

func ExecuteTest(t *testing.T, suite string) {
	var r []ginkgo.Reporter

	if dir := os.Getenv("TEST_REPORT_DIR"); len(dir) > 0 {
		TestContext.ReportDir = dir
	}

	if TestContext.ReportDir != "" {
		if err := os.MkdirAll(TestContext.ReportDir, 0755); err != nil {
			glog.Errorf("Failed creating report directory: %v", err)
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

	matches := make(map[string]*regexp.Regexp)
	for label, items := range testMaps {
		matches[label] = regexp.MustCompile(strings.Join(items, `|`))
	}

	ginkgo.WalkTests(func(name string, node types.TestNode) {
		labels := ""
		for {
			count := 0
			for label, matcher := range matches {
				if strings.Contains(name, label) {
					continue
				}
				if matcher.MatchString(name) {
					count++
					labels += " " + label
					name += " " + label
				}
			}
			if count == 0 {
				break
			}
		}
		if !excludedTestsFilter.MatchString(name) {
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
	})

	if quiet {
		r = append(r, NewSimpleReporter())
		ginkgo.RunSpecsWithCustomReporters(t, suite, r)
	} else {
		ginkgo.RunSpecsWithDefaultAndCustomReporters(t, suite, r)
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

func skipTestNamespaceCustomization() bool {
	return (isPackage("/kubernetes/test/e2e/namespace.go") && (testNameContains("should always delete fast") || testNameContains("should delete fast enough")))
}

// Holds custom namespace creation functions so we can customize per-test
var customCreateTestingNSFuncs = map[string]e2e.CreateTestingNSFn{}

// Registers a namespace creation function for the given basename
// Fails if a create function is already registered
func setCreateTestingNSFunc(baseName string, fn e2e.CreateTestingNSFn) {
	if _, exists := customCreateTestingNSFuncs[baseName]; exists {
		FatalErr("Double registered custom namespace creation function for " + baseName)
	}
	customCreateTestingNSFuncs[baseName] = fn
}

// createTestingNS delegates to custom namespace creation functions if registered.
// otherwise, it ensures that kubernetes e2e tests have their service accounts in the privileged and anyuid SCCs
func createTestingNS(baseName string, c kclientset.Interface, labels map[string]string) (*kapiv1.Namespace, error) {
	// If a custom function exists, call it
	if fn, exists := customCreateTestingNSFuncs[baseName]; exists {
		return fn(baseName, c, labels)
	}

	// Otherwise use the upstream default
	ns, err := e2e.CreateTestingNS(baseName, c, labels)
	if err != nil {
		return ns, err
	}

	// Add anyuid and privileged permissions for upstream tests
	if isKubernetesE2ETest() && !skipTestNamespaceCustomization() {
		clientConfig, err := testutil.GetClusterAdminClientConfig(KubeConfigPath())
		if err != nil {
			return ns, err
		}
		securityClient, err := securityclient.NewForConfig(clientConfig)
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
		addRoleToE2EServiceAccounts(rbacClient, []kapiv1.Namespace{*ns}, bootstrappolicy.ViewRoleName)

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
			`\[Feature:Initializers\]`,                       // admission controller disabled
			`\[Feature:LocalPersistentVolumes\]`,             // flag gate is off
			`\[Feature:PodPreemption\]`,                      // flag gate is off
			`\[Feature:RunAsGroup\]`,                         // flag gate is off
			`\[NodeAlphaFeature:VolumeSubpathEnvExpansion\]`, // flag gate is off
			`AdmissionWebhook`,                               // needs to be enabled
		},
		// tests for features that are not implemented in openshift
		"[Disabled:Unimplemented]": {
			`\[Feature:Networking-IPv6\]`, // openshift-sdn doesn't support yet
			`Monitoring`,                  // Not installed, should be
			`Cluster level logging`,       // Not installed yet
			`Kibana`,                      // Not installed
			`Ubernetes`,                   // Can't set zone labels today
			`kube-ui`,                     // Not installed by default
			`^Kubernetes Dashboard`,       // Not installed by default (also probably slow image pull)
			`Ingress`,                     // Not enabled yet

			`NetworkPolicy between server and client should allow egress access on one named port`, // not yet implemented

			`should proxy to cadvisor`, // we don't expose cAdvisor port directly for security reasons
		},
		// tests that rely on special configuration that we do not yet support
		"[Disabled:SpecialConfig]": {
			`\[Feature:ImageQuota\]`,                    // Quota isn't turned on by default, we should do that and then reenable these tests
			`\[Feature:Audit\]`,                         // Needs special configuration
			`\[Feature:LocalStorageCapacityIsolation\]`, // relies on a separate daemonset?

			`kube-dns-autoscaler`,                                                    // Don't run kube-dns
			`should check if Kubernetes master services is included in cluster-info`, // Don't run kube-dns
			`DNS configMap`, // this tests dns federation configuration via configmap, which we don't support yet

			// vSphere tests can be skipped generally
			`vsphere`,
			`Cinder`, // requires an OpenStack cluster
			// See the CanSupport implementation in upstream to determine wether these work.
			`Ceph RBD`,                              // Works if ceph-common Binary installed (but we can't guarantee this on all clusters).
			`GlusterFS`,                             // May work if /sbin/mount.glusterfs to be installed for plugin to work (also possibly blocked by serial pulling)
			`Horizontal pod autoscaling`,            // needs heapster
			`authentication: OpenLDAP`,              // needs separate setup and bucketing for openldap bootstrapping
			`NodeProblemDetector`,                   // requires a non-master node to run on
			`Advanced Audit should audit API calls`, // expects to be able to call /logs

			`Metadata Concealment`, // TODO: would be good to use

			`Firewall rule should have correct firewall rules for e2e cluster`, // Upstream-install specific
		},
		// tests that are known broken and need to be fixed upstream or in openshift
		// always add an issue here
		"[Disabled:Broken]": {
			`EmptyDir wrapper volumes should not conflict`,                   // uses git volume https://bugzilla.redhat.com/show_bug.cgi?id=1622195
			`\[Feature:BlockVolume\]`,                                        // directory failure https://bugzilla.redhat.com/show_bug.cgi?id=1622193
			`\[Feature:Example\]`,                                            // has cleanup issues
			`mount an API token into pods`,                                   // We add 6 secrets, not 1
			`ServiceAccounts should ensure a single API token exists`,        // We create lots of secrets
			`should test kube-proxy`,                                         // needs 2 nodes
			`unchanging, static URL paths for kubernetes api services`,       // the test needs to exclude URLs that are not part of conformance (/logs)
			"PersistentVolumes NFS when invoking the Recycle reclaim policy", // failing for some reason
			`should propagate mounts to the host`,                            // https://github.com/openshift/origin/issues/18931
			`Simple pod should handle in-cluster config`,                     // kubectl cp is not preserving executable bit
			`Services should be able to up and down services`,                // we don't have wget installed on nodes
			`Network should set TCP CLOSE_WAIT timeout`,                      // possibly some difference between ubuntu and fedora
			`should allow ingress access on one named port`,                  // broken even with network policy on

			`CSI plugin test using CSI driver: hostPath should provision storage`, // hangs waiting for binding, csi-pod doesn't start https://bugzilla.redhat.com/show_bug.cgi?id=1622670

			`\[NodeFeature:Sysctls\]`, // needs SCC support

			`validates that there is no conflict between pods with same hostPort but different hostIP and protocol`, // https://github.com/kubernetes/kubernetes/issues/61018

			`SSH`,                // TRIAGE
			`SELinux relabeling`, // https://github.com/openshift/origin/issues/7287 still broken
			`Volumes CephFS`,     // permission denied, selinux?

			`should support inline execution and attach`, // https://bugzilla.redhat.com/show_bug.cgi?id=1624041

			`should idle the service and DeploymentConfig properly`, // idling with a single service and DeploymentConfig [Conformance]
		},
		// tests too slow to be part of conformance
		"[Slow]": {
			`\[sig-scalability\]`,                          // disable from the default set for now
			`should create and stop a working application`, // Inordinately slow tests

			`\[Feature:PerformanceDNS\]`, // very slow

			`should ensure that critical pod is scheduled in case there is no resources available`, // should be tagged disruptive, consumes 100% of cluster CPU

			"Pod should avoid to schedule to node that have avoidPod annotation",
			"Pod should be schedule to node that satisify the PodAffinity",
			"Pod should be prefer scheduled to node that satisify the NodeAffinity",
			"Pod should be schedule to node that don't match the PodAntiAffinity terms", // 2m

			"validates that there exists conflict between pods with same hostPort and protocol but one using 0.0.0.0 hostIP", // 5m, really?
		},
		// tests that are known flaky
		"[Flaky]": {
			`Job should run a job to completion when tasks sometimes fail and are not locally restarted`, // seems flaky, also may require too many resources
			`openshift mongodb replication creating from a template`,                                     // flaking on deployment
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

			`Should be able to support the 1.7 Sample API Server using the current Aggregator`, // down apiservices break other clients today https://bugzilla.redhat.com/show_bug.cgi?id=1623195
		},
	}

	excludedTests = []string{
		`\[Disabled:.+\]`,
		`\[Disruptive\]`,
		`\[Skipped\]`,
		`\[Slow\]`,
		`\[Flaky\]`,
		`\[local\]`,
		`\[Local\]`,
	}
	excludedTestsFilter = regexp.MustCompile(strings.Join(excludedTests, `|`))
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
		ns.Annotations["openshift.io/node-selector"] = ""
		_, err = c.CoreV1().Namespaces().Update(ns)
		return err
	})
	if err != nil {
		FatalErr(err)
	}
}

func addE2EServiceAccountsToSCC(securityClient securityclient.Interface, namespaces []kapiv1.Namespace, sccName string) {
	// Because updates can race, we need to set the backoff retries to be > than the number of possible
	// parallel jobs starting at once. Set very high to allow future high parallelism.
	err := retry.RetryOnConflict(longRetry, func() error {
		scc, err := securityClient.Security().SecurityContextConstraints().Get(sccName, metav1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				return nil
			}
			return err
		}

		for _, ns := range namespaces {
			if strings.HasPrefix(ns.Name, "e2e-") {
				scc.Groups = append(scc.Groups, fmt.Sprintf("system:serviceaccounts:%s", ns.Name))
			}
		}
		if _, err := securityClient.Security().SecurityContextConstraints().Update(scc); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		FatalErr(err)
	}
}

func addRoleToE2EServiceAccounts(rbacClient rbacv1client.RbacV1Interface, namespaces []kapiv1.Namespace, roleName string) {
	err := retry.RetryOnConflict(longRetry, func() error {
		for _, ns := range namespaces {
			if strings.HasPrefix(ns.Name, "e2e-") && ns.Status.Phase != kapiv1.NamespaceTerminating {
				sa := fmt.Sprintf("system:serviceaccount:%s:default", ns.Name)
				addRole := &policy.RoleModificationOptions{
					RoleBindingNamespace: ns.Name,
					RoleKind:             "ClusterRole",
					RoleName:             roleName,
					RbacClient:           rbacClient,
					Users:                []string{sa},
				}
				if err := addRole.AddRole(); err != nil {
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
