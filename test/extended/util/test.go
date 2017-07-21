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
	"github.com/onsi/gomega"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/pkg/client/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/admin/policy"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/security/legacyclient"
	"github.com/openshift/origin/pkg/version"
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
	flag.StringVar(&syntheticSuite, "suite", "", "Optional suite selector to filter which tests are run.")

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

	// Disable density test unless it's explicitly requested.
	if config.GinkgoConfig.FocusString == "" && config.GinkgoConfig.SkipString == "" {
		config.GinkgoConfig.SkipString = "Skipped"
	}
	gomega.RegisterFailHandler(ginkgo.Fail)

	if reportDir != "" {
		r = append(r, reporters.NewJUnitReporter(path.Join(reportDir, fmt.Sprintf("%s_%02d.xml", reportFileName, config.GinkgoConfig.ParallelNode))))
	}

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
		e2e.Logf("About to run a Kube e2e test, ensuring namespace is privileged")
		// add the "privileged" scc to ensure pods that explicitly
		// request extra capabilities are not rejected
		addE2EServiceAccountsToSCC(c, []kapiv1.Namespace{*ns}, "privileged")
		// add the "anyuid" scc to ensure pods that don't specify a
		// uid don't get forced into a range (mimics upstream
		// behavior)
		addE2EServiceAccountsToSCC(c, []kapiv1.Namespace{*ns}, "anyuid")
		// add the "hostmount-anyuid" scc to ensure pods using hostPath
		// can execute tests
		addE2EServiceAccountsToSCC(c, []kapiv1.Namespace{*ns}, "hostmount-anyuid")

		// The intra-pod test requires that the service account have
		// permission to retrieve service endpoints.
		osClient, _, err := configapi.GetOpenShiftClient(KubeConfigPath(), nil)
		if err != nil {
			return ns, err
		}
		addRoleToE2EServiceAccounts(osClient, []kapiv1.Namespace{*ns}, bootstrappolicy.ViewRoleName)
	}

	// some test suites assume they can schedule to all nodes
	switch {
	case isPackage("/kubernetes/test/e2e/scheduler_predicates.go"),
		isPackage("/kubernetes/test/e2e/rescheduler.go"),
		isPackage("/kubernetes/test/e2e/kubelet.go"),
		isPackage("/kubernetes/test/e2e/common/networking.go"),
		isPackage("/kubernetes/test/e2e/daemon_set.go"),
		isPackage("/kubernetes/test/e2e/statefulset.go"):
		allowAllNodeScheduling(c, ns.Name)
	}

	return ns, err
}

var (
	excludedTests = []string{
		`\[Skipped\]`,
		`\[Disruptive\]`,
		`\[Slow\]`,
		`\[Flaky\]`,
		`\[Compatibility\]`,

		`\[Feature:Performance\]`,

		// not enabled in Origin yet
		`\[Feature:GarbageCollector\]`,

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
		`\[Feature:ManualPerformance\]`,                                // requires /resetMetrics which we don't expose

		// See the CanSupport implementation in upstream to determine wether these work.
		`Ceph RBD`,           // Works if ceph-common Binary installed (but we can't guarantee this on all clusters).
		`GlusterFS`,          // May work if /sbin/mount.glusterfs to be installed for plugin to work (also possibly blocked by serial pulling)
		`should support r/w`, // hostPath: This test expects that host's tmp dir is WRITABLE by a container.  That isn't something we need to guarantee for openshift.

		// Failing because of https://github.com/openshift/origin/issues/12365 against a real cluster
		`should allow starting 95 pods per node`,

		// Need fixing
		`Horizontal pod autoscaling`,                              // needs heapster
		`PersistentVolume`,                                        // https://github.com/openshift/origin/pull/6884 for recycler
		`mount an API token into pods`,                            // We add 6 secrets, not 1
		`ServiceAccounts should ensure a single API token exists`, // We create lots of secrets
		`Networking should function for intra-pod`,                // Needs two nodes, add equiv test for 1 node, then use networking suite
		`should test kube-proxy`,                                  // needs 2 nodes
		`authentication: OpenLDAP`,                                // needs separate setup and bucketing for openldap bootstrapping
		`NFS`, // no permissions https://github.com/openshift/origin/pull/6884
		`\[Feature:Example\]`,                                      // may need to pre-pull images
		`NodeProblemDetector`,                                      // requires a non-master node to run on
		`unchanging, static URL paths for kubernetes api services`, // the test needs to exclude URLs that are not part of conformance (/logs)

		// Needs triage to determine why it is failing
		`Addon update`, // TRIAGE
		`SSH`,          // TRIAGE
		`\[Feature:Upgrade\]`,                                                // TRIAGE
		`SELinux relabeling`,                                                 // started failing
		`openshift mongodb replication creating from a template`,             // flaking on deployment
		`Update Demo should do a rolling update of a replication controller`, // this is flaky and needs triaging

		// Test will never work
		`should proxy to cadvisor`, // we don't expose cAdvisor port directly for security reasons

		// Need to relax security restrictions
		`validates that InterPod Affinity and AntiAffinity is respected if matching`, // this *may* now be safe

		// Requires too many pods per node for the per core defaults
		`should ensure that critical pod is scheduled in case there is no resources available`,

		// Need multiple nodes
		`validates that InterPodAntiAffinity is respected if matching 2`,

		// Inordinately slow tests
		`should create and stop a working application`,
		`should always delete fast`, // will be uncommented in etcd3

		// tested by networking.sh and requires the environment that script sets up
		`\[networking\] OVS`,

		// We don't install KubeDNS
		`should check if Kubernetes master services is included in cluster-info`,

		// this tests dns federation configuration via configmap, which we don't support yet
		`DNS configMap`,

		// this tests the _kube_ downgrade. we don't support that.
		`\[Feature:Downgrade\]`,

		// upstream flakes
		`should provide basic identity`,                             // Basic StatefulSet functionality
		`validates resource limits of pods that are allowed to run`, // SchedulerPredicates
		`should idle the service and DeploymentConfig properly`,     // idling with a single service and DeploymentConfig [Conformance]

		// fails without a cloud provider
		"should be able to create a functioning NodePort service",

		// TODO undisable:
		"should be schedule to node that don't match the PodAntiAffinity terms",
		"should perfer to scheduled to nodes pod can tolerate",
		"should adopt matching orphans and release non-matching pods",
		"should not deadlock when a pod's predecessor fails",

		// slow as sin and twice as ugly (11m each)
		"Pod should avoid to schedule to node that have avoidPod annotation",
		"Pod should be schedule to node that satisify the PodAffinity",
		"Pod should be prefer scheduled to node that satisify the NodeAffinity",
	}
	excludedTestsFilter = regexp.MustCompile(strings.Join(excludedTests, `|`))

	parallelConformanceTests = []string{
		`\[Conformance\]`,
		`Services.*NodePort`,
		`ResourceQuota should`,
		`EmptyDir`,
		`StatefulSet`,
		`Downward API`,
		`DNS for ExternalName services`,
		`DNS for pods for Hostname and Subdomain annotation`,
		`PrivilegedPod should test privileged pod`,
		`Pods should support remote command execution`,
		`Pods should support retrieving logs from the container`,
		`Kubectl client Simple pod should support`,
		`Job should run a job to completion when tasks succeed`,
		`Variable Expansion`,
		`init containers`,
		`Clean up pods on node kubelet`,
		`\[Feature\:SecurityContext\]`,
		`should create a LimitRange with defaults`,
		`Generated release_1_2 clientset`,
		`should create a pod that reads a secret`,
		`should create a pod that prints his name and namespace`,
		`ImageLookup`,
		`DNS for pods for Hostname and Subdomain Annotation`,
	}
	parallelConformanceTestsFilter = regexp.MustCompile(strings.Join(parallelConformanceTests, `|`))

	serialConformanceTests = []string{
		`\[Serial\]`,
		`\[Feature:ManualPerformance\]`,      // requires isolation
		`Service endpoints latency`,          // requires low latency
		`\[Feature:HighDensityPerformance\]`, // requires no other namespaces
		`Clean up pods on node`,              // schedules max pods per node
	}
	serialConformanceTestsFilter = regexp.MustCompile(strings.Join(serialConformanceTests, `|`))
)

// checkSyntheticInput selects tests based on synthetic skips or focuses
func checkSyntheticInput() {
	checkSuiteFocuses()
	checkSuiteSkips()
}

// checkSuiteFocuses ensures Origin conformance suite synthetic labels are applied
func checkSuiteFocuses() {
	if !strings.Contains(syntheticSuite, "conformance.openshift.io") {
		return
	}

	testName := []byte(ginkgo.CurrentGinkgoTestDescription().FullTestText)
	testFocused := false
	textExcluded := excludedTestsFilter.Match(testName)
	if syntheticSuite == "parallel.conformance.openshift.io" {
		testFocused = parallelConformanceTestsFilter.Match(testName)
		textExcluded = textExcluded || serialConformanceTestsFilter.Match(testName)
	} else if syntheticSuite == "serial.conformance.openshift.io" {
		testFocused = serialConformanceTestsFilter.Match(testName)
	}

	if !testFocused || textExcluded {
		ginkgo.Skip("skipping tests not in the Origin conformance suite")
	}
}

// checkSuiteSkips ensures Origin/Kubernetes synthetic skip labels are applied
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

func addE2EServiceAccountsToSCC(c kclientset.Interface, namespaces []kapiv1.Namespace, sccName string) {
	// Because updates can race, we need to set the backoff retries to be > than the number of possible
	// parallel jobs starting at once. Set very high to allow future high parallelism.
	err := retry.RetryOnConflict(longRetry, func() error {
		scc, err := legacyclient.NewVersionedFromClient(c.Core().RESTClient()).Get(sccName, metav1.GetOptions{})
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
		if _, err := legacyclient.NewVersionedFromClient(c.Core().RESTClient()).Update(scc); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		FatalErr(err)
	}
}

func addRoleToE2EServiceAccounts(c *client.Client, namespaces []kapiv1.Namespace, roleName string) {
	err := retry.RetryOnConflict(longRetry, func() error {
		for _, ns := range namespaces {
			if strings.HasPrefix(ns.Name, "e2e-") && ns.Status.Phase != kapiv1.NamespaceTerminating {
				sa := fmt.Sprintf("system:serviceaccount:%s:default", ns.Name)
				addRole := &policy.RoleModificationOptions{
					RoleNamespace:       "",
					RoleName:            roleName,
					RoleBindingAccessor: policy.NewLocalRoleBindingAccessor(ns.Name, c),
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
