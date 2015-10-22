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

	apierrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/test/e2e"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
)

// init initialize the extended testing suite.
// You can set these environment variables to configure extended tests:
// KUBECONFIG - Path to kubeconfig containing embedded authinfo
func InitTest() {
	// Turn on verbose by default to get spec names
	config.DefaultReporterConfig.Verbose = false

	// Turn on EmitSpecProgress to get spec progress (especially on interrupt)
	config.GinkgoConfig.EmitSpecProgress = false

	// Randomize specs as well as suites
	config.GinkgoConfig.RandomizeAllSpecs = false

	extendedOutputDir := filepath.Join(os.TempDir(), "openshift-extended-tests")
	os.MkdirAll(extendedOutputDir, 0777)

	TestContext.VerifyServiceAccount = true
	TestContext.RepoRoot = os.Getenv("KUBE_REPO_ROOT")
	TestContext.KubectlPath = "kubectl"
	TestContext.KubeConfig = KubeConfigPath()
	os.Setenv("KUBECONFIG", TestContext.KubeConfig)

	//flag.StringVar(&TestContext.KubeConfig, clientcmd.RecommendedConfigPathFlag, KubeConfigPath(), "Path to kubeconfig containing embedded authinfo.")
	flag.StringVar(&TestContext.OutputDir, "extended-tests-output-dir", extendedOutputDir, "Output directory for interesting/useful test data, like performance data, benchmarks, and other metrics.")

	// Ensure that Kube tests run privileged (like they do upstream)
	ginkgo.JustBeforeEach(ensureKubeE2EPrivilegedSA)

	// Override the default Kubernetes E2E configuration
	e2e.SetTestContext(TestContext)
}

func ExecuteTest(t *testing.T, reportDir string) {
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
		r = append(r, reporters.NewJUnitReporter(path.Join(reportDir, fmt.Sprintf("junit_%02d.xml", config.GinkgoConfig.ParallelNode))))
	}

	r = append(r, NewSimpleReporter())

	ginkgo.RunSpecsWithCustomReporters(t, "OpenShift extended tests suite", r)
}

// ensureKubeE2EPrivilegedSA ensures that all namespaces prefixed with 'e2e-' have their
// service accounts in the privileged SCC
func ensureKubeE2EPrivilegedSA() {
	desc := ginkgo.CurrentGinkgoTestDescription()
	if strings.Contains(desc.FileName, "/kubernetes/test/e2e/") {
		e2e.Logf("About to run a Kube e2e test, ensuring namespace is privileged")
		c, _, err := configapi.GetKubeClient(KubeConfigPath())
		if err != nil {
			FatalErr(err)
		}
		priv, err := c.SecurityContextConstraints().Get("privileged")
		if err != nil {
			if apierrs.IsNotFound(err) {
				return
			}
			FatalErr(err)
		}
		namespaces, err := c.Namespaces().List(labels.Everything(), fields.Everything())
		if err != nil {
			FatalErr(err)
		}
		groups := []string{}
		for _, name := range priv.Groups {
			if !strings.Contains(name, "e2e-") {
				groups = append(groups, name)
			}
		}
		for _, ns := range namespaces.Items {
			if strings.HasPrefix(ns.Name, "e2e-") {
				groups = append(groups, fmt.Sprintf("system:serviceaccounts:%s", ns.Name))
			}
		}
		priv.Groups = groups
		if _, err := c.SecurityContextConstraints().Update(priv); err != nil {
			FatalErr(err)
		}
	}
}
