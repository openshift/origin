package util

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/golang/glog"
	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	"github.com/onsi/gomega"
	flag "github.com/spf13/pflag"
	//"k8s.io/kubernetes/pkg/client/clientcmd"
	"k8s.io/kubernetes/test/e2e"
)

// init initialize the extended testing suite.
// You can set these environment variables to configure extended tests:
// KUBECONFIG - Path to kubeconfig containing embedded authinfo
func InitTest() {
	// Turn on verbose by default to get spec names
	config.DefaultReporterConfig.Verbose = true

	// Turn on EmitSpecProgress to get spec progress (especially on interrupt)
	config.GinkgoConfig.EmitSpecProgress = true

	// Randomize specs as well as suites
	config.GinkgoConfig.RandomizeAllSpecs = false

	extendedOutputDir := filepath.Join(os.TempDir(), "openshift-extended-tests")
	os.MkdirAll(extendedOutputDir, 0777)

	TestContext.RepoRoot = os.Getenv("KUBE_REPO_ROOT")
	TestContext.KubectlPath = "oc"
	TestContext.KubeConfig = KubeConfigPath()
	os.Setenv("KUBECONFIG", TestContext.KubeConfig)

	//flag.StringVar(&TestContext.KubeConfig, clientcmd.RecommendedConfigPathFlag, KubeConfigPath(), "Path to kubeconfig containing embedded authinfo.")
	flag.StringVar(&TestContext.OutputDir, "extended-tests-output-dir", extendedOutputDir, "Output directory for interesting/useful test data, like performance data, benchmarks, and other metrics.")

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

	ginkgo.RunSpecsWithDefaultAndCustomReporters(t, "OpenShift extended tests suite", r)
}
