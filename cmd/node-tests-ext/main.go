package main

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/openshift-eng/openshift-tests-extension/pkg/cmd"
	"github.com/openshift-eng/openshift-tests-extension/pkg/dbtime"
	"github.com/openshift-eng/openshift-tests-extension/pkg/extension"
	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	"github.com/spf13/cobra"

	// Import the component tests
	_ "github.com/openshift/origin/test/extended/node"
)

var (
	CommitFromGit string
	BuildDate     string
	GitTreeState  string
)

// GinkgoTestingT implements the minimal TestingT interface needed by Ginkgo
type GinkgoTestingT struct{}

func (GinkgoTestingT) Errorf(format string, args ...interface{}) {}
func (GinkgoTestingT) Fail()                                     {}
func (GinkgoTestingT) FailNow()                                  { os.Exit(1) }

// NewGinkgoTestingT creates a new testing.T compatible instance for Ginkgo
func NewGinkgoTestingT() *GinkgoTestingT {
	return &GinkgoTestingT{}
}

// escapeRegexChars escapes special regex characters in test names for Ginkgo focus
func escapeRegexChars(s string) string {
	// Only escape the problematic characters that cause regex parsing issues
	// We need to escape [ and ] which are treated as character classes
	s = strings.ReplaceAll(s, "[", "\\[")
	s = strings.ReplaceAll(s, "]", "\\]")
	return s
}

// createTestSpec creates a test spec with proper execution functions
func createTestSpec(name, source string, codeLocations []string) *extensiontests.ExtensionTestSpec {
	return &extensiontests.ExtensionTestSpec{
		Name:          name,
		Source:        source,
		CodeLocations: codeLocations,
		Lifecycle:     extensiontests.LifecycleBlocking,
		Resources: extensiontests.Resources{
			Isolation: extensiontests.Isolation{},
		},
		EnvironmentSelector: extensiontests.EnvironmentSelector{},
		Run: func(ctx context.Context) *extensiontests.ExtensionTestResult {
			return runGinkgoTest(ctx, name)
		},
		RunParallel: func(ctx context.Context) *extensiontests.ExtensionTestResult {
			return runGinkgoTest(ctx, name)
		},
	}
}

// runGinkgoTest runs a Ginkgo test in-process
func runGinkgoTest(ctx context.Context, testName string) *extensiontests.ExtensionTestResult {
	startTime := time.Now()

	// Configure Ginkgo to run specific test
	gomega.RegisterFailHandler(ginkgo.Fail)

	// Run the test suite with focus on specific test
	suiteConfig, reporterConfig := ginkgo.GinkgoConfiguration()
	suiteConfig.FocusStrings = []string{escapeRegexChars(testName)}

	passed := ginkgo.RunSpecs(NewGinkgoTestingT(), "OpenShift node Test Suite", suiteConfig, reporterConfig)

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	result := extensiontests.ResultPassed
	if !passed {
		result = extensiontests.ResultFailed
	}

	return &extensiontests.ExtensionTestResult{
		Name:      testName,
		Result:    result,
		StartTime: dbtime.Ptr(startTime),
		EndTime:   dbtime.Ptr(endTime),
		Duration:  int64(duration.Seconds()),
		Output:    "",
	}
}

func main() {
	// Create a new registry
	registry := extension.NewRegistry()

	// Create extension for this component
	ext := extension.NewExtension("openshift", "payload", "node")

	// Set source information
	ext.Source = extension.Source{
		Commit:       CommitFromGit,
		BuildDate:    BuildDate,
		GitTreeState: GitTreeState,
	}

	// Add test suites for node
	ext.AddGlobalSuite(extension.Suite{
		Name:        "openshift/node/conformance/parallel",
		Description: "",
		Parents:     []string{"openshift/conformance/parallel"},
		Qualifiers:  []string{"(source == \"openshift:payload:node\") && (!(name.contains(\"[Serial]\") || name.contains(\"[Slow]\")))"},
	})

	ext.AddGlobalSuite(extension.Suite{
		Name:        "openshift/node/conformance/serial",
		Description: "",
		Parents:     []string{"openshift/conformance/serial"},
		Qualifiers:  []string{"(source == \"openshift:payload:node\") && (name.contains(\"[Serial]\"))"},
	})

	ext.AddGlobalSuite(extension.Suite{
		Name:        "openshift/node/optional/slow",
		Description: "",
		Parents:     []string{"openshift/optional/slow"},
		Qualifiers:  []string{"(source == \"openshift:payload:node\") && (name.contains(\"[Slow]\"))"},
	})

	ext.AddGlobalSuite(extension.Suite{
		Name:        "openshift/node/all",
		Description: "",
		Qualifiers:  []string{"source == \"openshift:payload:node\""},
	})

	// Add sample test spec (you'll need to add actual tests based on existing test/extended/node/)
	testSpecs := extensiontests.ExtensionTestSpecs{
		createTestSpec(
			"[Jira:node][sig-node] node test should always pass [Suite:openshift/node/conformance/parallel]",
			"openshift:payload:node",
			[]string{
				"/test/extended/node/node.go:10",
				"/test/extended/node/node.go:11",
			},
		),
	}
	ext.AddSpecs(testSpecs)

	// Register the extension
	registry.Register(ext)

	// Create root command with default extension commands
	rootCmd := &cobra.Command{
		Use:   "node-tests-ext",
		Short: "OpenShift node tests extension",
	}

	// Add all the default extension commands (info, list, run-test, run-suite, update)
	rootCmd.AddCommand(cmd.DefaultExtensionCommands(registry)...)

	// Execute the command
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
