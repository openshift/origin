package util

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang/glog"
	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	"github.com/onsi/gomega"
	flag "github.com/spf13/pflag"

	kapi "k8s.io/kubernetes/pkg/api"
	apierrs "k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/admin/policy"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

var (
	reportDir      string
	reportFileName string
	quiet          bool
)

var TestContext *e2e.TestContextType = &e2e.TestContext

// init initialize the extended testing suite.
// You can set these environment variables to configure extended tests:
// KUBECONFIG - Path to kubeconfig containing embedded authinfo
// TEST_REPORT_DIR - If set, JUnit output will be written to this directory for each test
// TEST_REPORT_FILE_NAME - If set, will determine the name of the file that JUnit output is written to
func InitTest() {
	// Add hooks to skip all kubernetes or origin tests
	ginkgo.BeforeEach(checkSuiteSkips)

	e2e.RegisterCommonFlags()
	e2e.RegisterClusterFlags()

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
func createTestingNS(baseName string, c *kclient.Client, labels map[string]string) (*kapi.Namespace, error) {
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
		// add to the "privileged" scc to ensure pods that explicitly
		// request extra capabilities are not rejected
		addE2EServiceAccountsToSCC(c, []kapi.Namespace{*ns}, "privileged")
		// add to the "anyuid" scc to ensure pods that don't specify a
		// uid don't get forced into a range (mimics upstream
		// behavior)
		addE2EServiceAccountsToSCC(c, []kapi.Namespace{*ns}, "anyuid")

		// The intra-pod test requires that the service account have
		// permission to retrieve service endpoints.
		osClient, _, err := configapi.GetOpenShiftClient(KubeConfigPath(), nil)
		if err != nil {
			return ns, err
		}
		addRoleToE2EServiceAccounts(osClient, []kapi.Namespace{*ns}, bootstrappolicy.ViewRoleName)
	}

	return ns, err
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

func addE2EServiceAccountsToSCC(c *kclient.Client, namespaces []kapi.Namespace, sccName string) {
	err := kclient.RetryOnConflict(kclient.DefaultRetry, func() error {
		scc, err := c.SecurityContextConstraints().Get(sccName)
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
		if _, err := c.SecurityContextConstraints().Update(scc); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		FatalErr(err)
	}
}

func addRoleToE2EServiceAccounts(c *client.Client, namespaces []kapi.Namespace, roleName string) {
	err := kclient.RetryOnConflict(kclient.DefaultRetry, func() error {
		for _, ns := range namespaces {
			if strings.HasPrefix(ns.Name, "e2e-") && ns.Status.Phase != kapi.NamespaceTerminating {
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
