package util

import (
	rflag "flag"
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
	"k8s.io/kubernetes/test/e2e"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

var (
	reportDir      string
	reportFileName string
	quiet          bool
)

// init initialize the extended testing suite.
// You can set these environment variables to configure extended tests:
// KUBECONFIG - Path to kubeconfig containing embedded authinfo
// TEST_REPORT_DIR - If set, JUnit output will be written to this directory for each test
// TEST_REPORT_FILE_NAME - If set, will determine the name of the file that JUnit output is written to
func InitTest() {
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
	//flag.StringVar(&TestContext.KubeConfig, clientcmd.RecommendedConfigPathFlag, KubeConfigPath(), "Path to kubeconfig containing embedded authinfo.")
	flag.StringVar(&TestContext.OutputDir, "extended-tests-output-dir", extendedOutputDir, "Output directory for interesting/useful test data, like performance data, benchmarks, and other metrics.")
	rflag.StringVar(&config.GinkgoConfig.FocusString, "focus", "", "DEPRECATED: use --ginkgo.focus")

	// Ensure that Kube tests run privileged (like they do upstream)
	ginkgo.JustBeforeEach(verifyTestSuitePreconditions)

	// Override the default Kubernetes E2E configuration
	e2e.SetTestContext(TestContext)
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

// verifyTestSuitePreconditions ensures that all namespaces prefixed with 'e2e-' have their
// service accounts in the privileged and anyuid SCCs, and that Origin/Kubernetes synthetic
// skip labels are applied
func verifyTestSuitePreconditions() {
	desc := ginkgo.CurrentGinkgoTestDescription()

	switch {
	case strings.Contains(desc.FileName, "/origin/test/"):
		if strings.Contains(config.GinkgoConfig.SkipString, "[Origin]") {
			ginkgo.Skip("skipping [Origin] tests")
		}

	case strings.Contains(desc.FileName, "/kubernetes/test/e2e/"):
		if strings.Contains(config.GinkgoConfig.SkipString, "[Kubernetes]") {
			ginkgo.Skip("skipping [Kubernetes] tests")
		}

		e2e.Logf("About to run a Kube e2e test, ensuring namespace is privileged")
		c, _, err := configapi.GetKubeClient(KubeConfigPath())
		if err != nil {
			FatalErr(err)
		}
		namespaces, err := c.Namespaces().List(kapi.ListOptions{})
		if err != nil {
			FatalErr(err)
		}
		// add to the "privileged" scc to ensure pods that explicitly
		// request extra capabilities are not rejected
		addE2EServiceAccountsToSCC(c, namespaces, "privileged")
		// add to the "anyuid" scc to ensure pods that don't specify a
		// uid don't get forced into a range (mimics upstream
		// behavior)
		addE2EServiceAccountsToSCC(c, namespaces, "anyuid")
	}
}

func addE2EServiceAccountsToSCC(c *kclient.Client, namespaces *kapi.NamespaceList, sccName string) {
	err := kclient.RetryOnConflict(kclient.DefaultRetry, func() error {
		scc, err := c.SecurityContextConstraints().Get(sccName)
		if err != nil {
			if apierrs.IsNotFound(err) {
				return nil
			}
			return err
		}

		groups := []string{}
		for _, name := range scc.Groups {
			if !strings.Contains(name, "e2e-") {
				groups = append(groups, name)
			}
		}
		for _, ns := range namespaces.Items {
			if strings.HasPrefix(ns.Name, "e2e-") {
				groups = append(groups, fmt.Sprintf("system:serviceaccounts:%s", ns.Name))
			}
		}
		scc.Groups = groups
		if _, err := c.SecurityContextConstraints().Update(scc); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		FatalErr(err)
	}
}
