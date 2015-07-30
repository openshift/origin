package extended

import (
	"flag"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	. "github.com/GoogleCloudPlatform/kubernetes/test/e2e"
	"github.com/golang/glog"
	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	"github.com/onsi/gomega"
)

var (
	reportDir = flag.String("report-dir", "", "Path to the directory where the JUnit XML reports should be saved. Default is empty, which doesn't generate these reports.")
)

// init initialize the extended testing suite.
// You can set these environment variables to configure extended tests:
// MASTER_ADDR - The host or apiserver to connect to
// SERVER_CONFIG_DIR - Directory where OpenShift stores configuration
// SERVER_KUBECONFIG_PATH - Location of 'admin.kubeconfig' file
// SERVER_CERT_DIR - Directory with OpenShift client certificates
func init() {
	// Turn on verbose by default to get spec names
	config.DefaultReporterConfig.Verbose = true

	// Turn on EmitSpecProgress to get spec progress (especially on interrupt)
	config.GinkgoConfig.EmitSpecProgress = true

	// Randomize specs as well as suites
	config.GinkgoConfig.RandomizeAllSpecs = false

	flag.StringVar(&testContext.KubeConfig, clientcmd.RecommendedConfigPathFlag, adminKubeConfigPath(), "Path to kubeconfig containing embeded authinfo.")
	flag.StringVar(&testContext.KubeContext, clientcmd.FlagContext, "", "kubeconfig context to use/override. If unset, will use value from 'current-context'")
	flag.StringVar(&testContext.Host, "host", os.Getenv("MASTER_ADDR"), "The host, or apiserver, to connect to")
	flag.StringVar(&testContext.OutputDir, "extended-tests-output-dir", os.TempDir(), "Output directory for interesting/useful test data, like performance data, benchmarks, and other metrics.")

	// Override the default Kubernetes E2E configuration
	SetTestContext(testContext)
}

func TestExtended(t *testing.T) {
	var r []ginkgo.Reporter

	if *reportDir != "" {
		if err := os.MkdirAll(*reportDir, 0755); err != nil {
			glog.Errorf("Failed creating report directory: %v", err)
		}
		defer CoreDump(*reportDir)
	}

	// Disable density test unless it's explicitly requested.
	if config.GinkgoConfig.FocusString == "" && config.GinkgoConfig.SkipString == "" {
		config.GinkgoConfig.SkipString = "Skipped"
	}
	gomega.RegisterFailHandler(ginkgo.Fail)

	if *reportDir != "" {
		r = append(r, reporters.NewJUnitReporter(path.Join(*reportDir, fmt.Sprintf("junit_%02d.xml", config.GinkgoConfig.ParallelNode))))
	}

	ginkgo.RunSpecsWithDefaultAndCustomReporters(t, "OpenShift extended tests suite", r)
}
