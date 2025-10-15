package util

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"k8s.io/klog/v2"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kclientset "k8s.io/client-go/kubernetes"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/openshift-hack/e2e"
	conformancetestdata "k8s.io/kubernetes/test/conformance/testdata"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/testfiles"
	e2etestingmanifests "k8s.io/kubernetes/test/e2e/testing-manifests"
	testfixtures "k8s.io/kubernetes/test/fixtures"

	// this appears to inexplicably auto-register global flags.
	_ "k8s.io/kubernetes/test/e2e/storage/drivers"

	projectv1 "github.com/openshift/api/project/v1"
	securityv1client "github.com/openshift/client-go/security/clientset/versioned"

	"github.com/openshift/origin/pkg/version"
)

var (
	// TestContext which allows injecting context data into tests
	TestContext *framework.TestContextType = &framework.TestContext

	// upgradeFilter is meant for matching upgrade test's namespace
	upgradeFilter = regexp.MustCompile(`e2e-k8s[\w-]+upgrade`)
)

func InitStandardFlags() {
	framework.RegisterCommonFlags(flag.CommandLine)
	framework.RegisterClusterFlags(flag.CommandLine)

	// replaced by a bare import above.
	//framework.RegisterStorageFlags()
}

func InitTest(dryRun bool) error {
	InitDefaultEnvironmentVariables()

	// Set hypervisor configuration in TestContext if available
	hypervisorConfigJSON := os.Getenv("HYPERVISOR_CONFIG")
	if hypervisorConfigJSON != "" {
		// Parse and validate hypervisor configuration
		var hypervisorConfig struct {
			HypervisorIP   string `json:"hypervisorIP"`
			SSHUser        string `json:"sshUser"`
			PrivateKeyPath string `json:"privateKeyPath"`
		}
		if err := json.Unmarshal([]byte(hypervisorConfigJSON), &hypervisorConfig); err != nil {
			return fmt.Errorf("failed to parse hypervisor configuration JSON: %v", err)
		}

		// Validate required fields
		if hypervisorConfig.HypervisorIP == "" {
			return fmt.Errorf("hypervisorIP is required in hypervisor configuration")
		}
		if hypervisorConfig.SSHUser == "" {
			return fmt.Errorf("sshUser is required in hypervisor configuration")
		}
		if hypervisorConfig.PrivateKeyPath == "" {
			return fmt.Errorf("privateKeyPath is required in hypervisor configuration")
		}

		// Store the hypervisor configuration in TestContext for tests to access
		// We'll use the existing CloudConfig.ConfigFile field to store the JSON
		// This is a workaround since we can't extend TestContextType directly
		TestContext.CloudConfig.ConfigFile = hypervisorConfigJSON
	}

	TestContext.DeleteNamespace = os.Getenv("DELETE_NAMESPACE") != "false"
	TestContext.VerifyServiceAccount = true
	testfiles.AddFileSource(e2etestingmanifests.GetE2ETestingManifestsFS())
	testfiles.AddFileSource(testfixtures.GetTestFixturesFS())
	testfiles.AddFileSource(conformancetestdata.GetConformanceTestdataFS())
	TestContext.KubectlPath = "kubectl"
	TestContext.KubeConfig = KubeConfigPath()

	// "debian" is used when not set. At least GlusterFS tests need "custom".
	// (There is no option for "rhel" or "centos".)
	TestContext.NodeOSDistro = "custom"
	TestContext.MasterOSDistro = "custom"

	// load and set the host variable for kubectl
	if !dryRun {
		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(&clientcmd.ClientConfigLoadingRules{ExplicitPath: TestContext.KubeConfig}, &clientcmd.ConfigOverrides{})
		cfg, err := clientConfig.ClientConfig()
		if err != nil {
			return err
		}
		TestContext.Host = cfg.Host
	}
	// Ensure that Kube tests run privileged (like they do upstream)
	TestContext.CreateTestingNS = func(ctx context.Context, baseName string, c kclientset.Interface, labels map[string]string) (*corev1.Namespace, error) {
		// there are several cases when we want to kick in with namespace customization
		// which includes setting persmissions for upstream tests to allow for privileged
		// operations, these cases are:
		// 1. all tests which namespace matches `e2e-k8s-...-upgrade` pattern
		//    (mostly k8s upgrades tests, see test/extended/util/disruption/disruption.go#createTestFrameworks)
		// 2. all k8s tests (based on testfile location), which don't have specific wording in their name (see skipTestNamespaceCustomization)
		isKubeNamespace := upgradeFilter.MatchString(baseName) || // 1.
			(isGoModulePath(ginkgo.CurrentSpecReport().FileName(), "k8s.io/kubernetes", "test/e2e") && !skipTestNamespaceCustomization()) // 2.
		return e2e.CreateTestingNS(ctx, baseName, c, labels, isKubeNamespace)
	}

	klog.V(2).Infof("Extended test version %s", version.Get().String())
	return nil
}

var testsStarted bool

// requiresTestStart indicates this code should never be called from within init() or
// Ginkgo test definition.
//
// We explictly prevent Run() from outside of a test because it means that
// test initialization may be expensive. Tests should not vary definition
// based on a cluster, they should be static in definition. Always use framework.Skipf()
// if your test should not be run based on a dynamic condition of the cluster.
func requiresTestStart() {
	if !testsStarted {
		panic("May only be called from within a test case")
	}
}

// isGoModulePath returns true if the packagePath reported by reflection is within a
// module and given module path. When go mod is in use, module and modulePath are not
// contiguous as they were in older golang versions with vendoring, so naive contains
// tests fail.
//
// historically: ".../vendor/k8s.io/kubernetes/test/e2e"
// go.mod:       "k8s.io/kubernetes@0.18.4/test/e2e"
func isGoModulePath(packagePath, module, modulePath string) bool {
	return regexp.MustCompile(fmt.Sprintf(`\b%s(@[^/]*|)/%s\b`, regexp.QuoteMeta(module), regexp.QuoteMeta(modulePath))).MatchString(packagePath)
}

// skipTestNamespaceCustomization returns true for tests which have specific
// wording in their name and we know they should not be customized
func skipTestNamespaceCustomization() bool {
	testName := ginkgo.CurrentSpecReport().FullText()
	return strings.Contains(testName, "should always delete fast") || strings.Contains(testName, "should delete fast enough")
}

// WithCleanup instructs utility methods to move out of dry run mode so there are no side
// effects due to package initialization of Ginkgo tests, and then after the function
// completes cleans up any artifacts created by this project.
func WithCleanup(fn func()) {
	testsStarted = true

	// Initialize the fixture directory. If we were the ones to initialize it, set the env
	// var so that child processes inherit this directory and take responsibility for
	// cleaning it up after we exit.
	fixtureDir, init := fixtureDirectory()
	if init {
		os.Setenv("OS_TEST_FIXTURE_DIR", fixtureDir)
		defer func() {
			os.Setenv("OS_TEST_FIXTURE_DIR", "")
			os.RemoveAll(fixtureDir)
		}()
	}

	fn()
}

// InitDefaultEnvironmentVariables makes sure certain required env vars are available
// in the case that extended tests are invoked directly via calls to ginkgo/extended.test
func InitDefaultEnvironmentVariables() {
	if ad := os.Getenv("ARTIFACT_DIR"); len(strings.TrimSpace(ad)) == 0 {
		os.Setenv("ARTIFACT_DIR", filepath.Join(os.TempDir(), "artifacts"))
	}
}

var longRetry = wait.Backoff{Steps: 100}

// allowAllNodeScheduling sets the annotation on namespace that allows all nodes to be scheduled onto.
func allowAllNodeScheduling(c kclientset.Interface, namespace string) {
	err := retry.RetryOnConflict(longRetry, func() error {
		ns, err := c.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if ns.Annotations == nil {
			ns.Annotations = make(map[string]string)
		}
		ns.Annotations[projectv1.ProjectNodeSelector] = ""
		_, err = c.CoreV1().Namespaces().Update(context.Background(), ns, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		FatalErr(err)
	}
}

func addE2EServiceAccountsToSCC(securityClient securityv1client.Interface, namespaces []corev1.Namespace, sccName string) {
	// Because updates can race, we need to set the backoff retries to be > than the number of possible
	// parallel jobs starting at once. Set very high to allow future high parallelism.
	err := retry.RetryOnConflict(longRetry, func() error {
		scc, err := securityClient.SecurityV1().SecurityContextConstraints().Get(context.Background(), sccName, metav1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				return nil
			}
			return err
		}

		for _, ns := range namespaces {
			scc.Groups = append(scc.Groups, fmt.Sprintf("system:serviceaccounts:%s", ns.Name))
		}
		if _, err := securityClient.SecurityV1().SecurityContextConstraints().Update(context.Background(), scc, metav1.UpdateOptions{}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		FatalErr(err)
	}
}

func addRoleToE2EServiceAccounts(rbacClient rbacv1client.RbacV1Interface, namespaces []corev1.Namespace, roleName string) {
	err := retry.RetryOnConflict(longRetry, func() error {
		for _, ns := range namespaces {
			if ns.Status.Phase != corev1.NamespaceTerminating {
				_, err := rbacClient.RoleBindings(ns.Name).Create(context.Background(), &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{GenerateName: "default-" + roleName, Namespace: ns.Name},
					RoleRef: rbacv1.RoleRef{
						Kind: "ClusterRole",
						Name: roleName,
					},
					Subjects: []rbacv1.Subject{
						{Name: "default", Namespace: ns.Name, Kind: rbacv1.ServiceAccountKind},
					},
				}, metav1.CreateOptions{})
				if err != nil {
					framework.Logf("Warning: Failed to add role to e2e service account: %v", err)
				}
			}
		}
		return nil
	})
	if err != nil {
		FatalErr(err)
	}
}

// GetHypervisorConfig returns the hypervisor configuration if available
func GetHypervisorConfig() *struct {
	HypervisorIP   string `json:"hypervisorIP"`
	SSHUser        string `json:"sshUser"`
	PrivateKeyPath string `json:"privateKeyPath"`
} {
	hypervisorConfigJSON := TestContext.CloudConfig.ConfigFile
	if hypervisorConfigJSON == "" {
		return nil
	}

	var hypervisorConfig struct {
		HypervisorIP   string `json:"hypervisorIP"`
		SSHUser        string `json:"sshUser"`
		PrivateKeyPath string `json:"privateKeyPath"`
	}
	if err := json.Unmarshal([]byte(hypervisorConfigJSON), &hypervisorConfig); err != nil {
		return nil
	}

	return &hypervisorConfig
}

// HasHypervisorConfig returns true if hypervisor configuration is available
func HasHypervisorConfig() bool {
	return TestContext.CloudConfig.ConfigFile != ""
}
