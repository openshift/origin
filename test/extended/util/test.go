package util

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
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
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	authorizationclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/oc/admin/policy"
	securityclient "github.com/openshift/origin/pkg/security/generated/internalclientset"
	"github.com/openshift/origin/pkg/version"
	testutil "github.com/openshift/origin/test/util"
)

var (
	reportDir      string
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

	e2e.RegisterCommonFlags()
	e2e.RegisterClusterFlags()
	flag.StringVar(&syntheticSuite, "suite", "", "DEPRECATED: Optional suite selector to filter which tests are run. Use focus.")

	extendedOutputDir := filepath.Join(os.TempDir(), "openshift-extended-tests")
	os.MkdirAll(extendedOutputDir, 0777)

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

	reportDir = os.Getenv("TEST_REPORT_DIR")

	reportFileName = os.Getenv("TEST_REPORT_FILE_NAME")
	if reportFileName == "" {
		reportFileName = "junit"
	}

	quiet = os.Getenv("TEST_OUTPUT_QUIET") == "true"
	flag.StringVar(&TestContext.OutputDir, "extended-tests-output-dir", extendedOutputDir, "Output directory for interesting/useful test data, like performance data, benchmarks, and other metrics.")

	// Ensure that Kube tests run privileged (like they do upstream)
	TestContext.CreateTestingNS = createTestingNS

	glog.Infof("Extended test version %s", version.Get().String())
}

func ExecuteTest(t *testing.T, suite string) {
	var r []ginkgo.Reporter

	if reportDir != "" {
		if err := os.MkdirAll(reportDir, 0755); err != nil {
			glog.Errorf("Failed creating report directory: %v", err)
		}
		defer e2e.CoreDump(reportDir)
	}

	switch syntheticSuite {
	case "parallel.conformance.openshift.io":
		if len(config.GinkgoConfig.FocusString) > 0 {
			config.GinkgoConfig.FocusString += "|"
		}
		config.GinkgoConfig.FocusString = "\\[Suite:openshift/conformance/parallel\\]"
	case "serial.conformance.openshift.io":
		if len(config.GinkgoConfig.FocusString) > 0 {
			config.GinkgoConfig.FocusString += "|"
		}
		config.GinkgoConfig.FocusString = "\\[Suite:openshift/conformance/serial\\]"
	}
	if config.GinkgoConfig.FocusString == "" && config.GinkgoConfig.SkipString == "" {
		config.GinkgoConfig.SkipString = "Skipped"
	}

	gomega.RegisterFailHandler(ginkgo.Fail)

	if reportDir != "" {
		r = append(r, reporters.NewJUnitReporter(path.Join(reportDir, fmt.Sprintf("%s_%02d.xml", reportFileName, config.GinkgoConfig.ParallelNode))))
	}

	ginkgo.WalkTests(func(name string, node types.TestNode) {
		isSerial := serialTestsFilter.MatchString(name)
		if isSerial {
			if !strings.Contains(name, "[Serial]") {
				node.SetText(node.Text() + " [Serial]")
			}
		}

		if !excludedTestsFilter.MatchString(name) {
			include := conformanceTestsFilter.MatchString(name)
			switch {
			case !include:
				// do nothing
			case isSerial:
				node.SetText(node.Text() + " [Suite:openshift/conformance/serial]")
			case include:
				node.SetText(node.Text() + " [Suite:openshift/conformance/parallel]")
			}
		}
		if strings.Contains(node.CodeLocation().FileName, "/origin/test/") && !strings.Contains(node.Text(), "[Suite:openshift") {
			node.SetText(node.Text() + " [Suite:openshift]")
		}
		if strings.Contains(node.CodeLocation().FileName, "/kubernetes/test/e2e/") {
			node.SetText(node.Text() + " [Suite:k8s]")
		}
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
		authorizationClient, err := authorizationclient.NewForConfig(clientConfig)
		if err != nil {
			return ns, err
		}
		addRoleToE2EServiceAccounts(authorizationClient, []kapiv1.Namespace{*ns}, bootstrappolicy.ViewRoleName)

		// in practice too many kube tests ignore scheduling constraints
		allowAllNodeScheduling(c, ns.Name)
	}

	return ns, err
}

var (
	excludedTests = []string{
		`\[Skipped\]`,
		`\[Slow\]`,
		`\[Flaky\]`,
		`\[Disruptive\]`,
		`\[local\]`,

		// not enabled in Origin yet
		//`\[Feature:GarbageCollector\]`,

		// Doesn't work on scaled up clusters
		`\[Feature:ImagePrune\]`,
		// Quota isn't turned on by default, we should do that and then reenable these tests
		`\[Feature:ImageQuota\]`,
		// Currently disabled by default
		`\[Feature:Initializers\]`,
		// Needs special configuration
		`\[Feature:Audit\]`,

		// Depends on external components, may not need yet
		`Monitoring`,            // Not installed, should be
		`Cluster level logging`, // Not installed yet
		`Kibana`,                // Not installed
		`Ubernetes`,             // Can't set zone labels today
		`kube-ui`,               // Not installed by default
		`^Kubernetes Dashboard`, // Not installed by default (also probably slow image pull)

		`\[Feature:Federation\]`,                                       // Not enabled yet
		`\[Feature:Federation12\]`,                                     // Not enabled yet
		`Ingress`,                                                      // Not enabled yet
		`Cinder`,                                                       // requires an OpenStack cluster
		`should support r/w`,                                           // hostPath: This test expects that host's tmp dir is WRITABLE by a container.  That isn't something we need to guarantee for openshift.
		`should check that the kubernetes-dashboard instance is alive`, // we don't create this
		//		`\[Feature:ManualPerformance\]`,                                // requires /resetMetrics which we don't expose

		// See the CanSupport implementation in upstream to determine wether these work.
		`Ceph RBD`,           // Works if ceph-common Binary installed (but we can't guarantee this on all clusters).
		`GlusterFS`,          // May work if /sbin/mount.glusterfs to be installed for plugin to work (also possibly blocked by serial pulling)
		`should support r/w`, // hostPath: This test expects that host's tmp dir is WRITABLE by a container.  That isn't something we need to guarantee for openshift.

		// Failing because of https://github.com/openshift/origin/issues/12365 against a real cluster
		//`should allow starting 95 pods per node`,

		// Need fixing
		`Horizontal pod autoscaling`, // needs heapster
		//`PersistentVolume`,                                        // https://github.com/openshift/origin/pull/6884 for recycler
		`mount an API token into pods`,                            // We add 6 secrets, not 1
		`ServiceAccounts should ensure a single API token exists`, // We create lots of secrets
		`should test kube-proxy`,                                  // needs 2 nodes
		`authentication: OpenLDAP`,                                // needs separate setup and bucketing for openldap bootstrapping
		`NFS`, // no permissions https://github.com/openshift/origin/pull/6884
		`\[Feature:Example\]`, // has cleanup issues
		`NodeProblemDetector`, // requires a non-master node to run on
		//`unchanging, static URL paths for kubernetes api services`, // the test needs to exclude URLs that are not part of conformance (/logs)

		// Needs triage to determine why it is failing
		`Addon update`, // TRIAGE
		`SSH`,          // TRIAGE
		`\[Feature:Upgrade\]`,                                    // TRIAGE
		`SELinux relabeling`,                                     // https://github.com/openshift/origin/issues/7287
		`openshift mongodb replication creating from a template`, // flaking on deployment
		//`Update Demo should do a rolling update of a replication controller`, // this is flaky and needs triaging

		// Test will never work
		`should proxy to cadvisor`, // we don't expose cAdvisor port directly for security reasons

		// Need to relax security restrictions
		//`validates that InterPod Affinity and AntiAffinity is respected if matching`, // this *may* now be safe

		// Requires too many pods per node for the per core defaults
		//`should ensure that critical pod is scheduled in case there is no resources available`,

		// Need multiple nodes
		`validates that InterPodAntiAffinity is respected if matching 2`,

		// Inordinately slow tests
		`should create and stop a working application`,
		//`should always delete fast`, // will be uncommented in etcd3

		// We don't install KubeDNS
		`should check if Kubernetes master services is included in cluster-info`,

		// this tests dns federation configuration via configmap, which we don't support yet
		`DNS configMap`,

		// this tests the _kube_ downgrade. we don't support that.
		`\[Feature:Downgrade\]`,

		// upstream flakes
		`validates resource limits of pods that are allowed to run`, // can't schedule to master due to node label limits, also fiddly

		// TODO undisable:
		`should provide basic identity`,                         // needs a persistent volume provisioner in single node, host path not working
		`should idle the service and DeploymentConfig properly`, // idling with a single service and DeploymentConfig [Conformance]

		// slow as sin and twice as ugly (11m each)
		"Pod should avoid to schedule to node that have avoidPod annotation",
		"Pod should be schedule to node that satisify the PodAffinity",
		"Pod should be prefer scheduled to node that satisify the NodeAffinity",
	}
	excludedTestsFilter = regexp.MustCompile(strings.Join(excludedTests, `|`))

	// The list of tests to run for the OpenShift conformance suite. Any test
	// in this group which cannot be run in parallel must be identified with the
	// [Serial] tag or added to the serialTests filter.
	conformanceTests       = []string{}
	conformanceTestsFilter = regexp.MustCompile(strings.Join(conformanceTests, `|`))

	// Identifies any tests that by nature must be run in isolation. Every test in this
	// category will be given the [Serial] tag if it does not already have it.
	serialTests = []string{
		`\[Serial\]`,
		`\[Disruptive\]`,
		`\[Feature:ManualPerformance\]`,      // requires isolation
		`\[Feature:HighDensityPerformance\]`, // requires no other namespaces
		`Service endpoints latency`,          // requires low latency
		`Clean up pods on node`,              // schedules up to max pods per node
		`should allow starting 95 pods per node`,
	}
	serialTestsFilter = regexp.MustCompile(strings.Join(serialTests, `|`))
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

func addRoleToE2EServiceAccounts(c authorizationclient.Interface, namespaces []kapiv1.Namespace, roleName string) {
	err := retry.RetryOnConflict(longRetry, func() error {
		for _, ns := range namespaces {
			if strings.HasPrefix(ns.Name, "e2e-") && ns.Status.Phase != kapiv1.NamespaceTerminating {
				sa := fmt.Sprintf("system:serviceaccount:%s:default", ns.Name)
				addRole := &policy.RoleModificationOptions{
					RoleNamespace:       "",
					RoleName:            roleName,
					RoleBindingAccessor: policy.NewLocalRoleBindingAccessor(ns.Name, c.Authorization()),
					Users:               []string{sa},
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
