package e2e_analysis

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/stretchr/testify/require"
)

// TestReadinessChecksSkippedJUnitTestCases verifies skipped readiness checks are reported in JUnit output.
func TestReadinessChecksSkippedJUnitTestCases(t *testing.T) {
	tests := []struct {
		name string
		env  bool
		args []string
	}{
		{
			name: "environment variable",
			env:  true,
		},
		{
			name: "command line flag",
			args: []string{"--skip-readiness-checks"},
		},
	}

	expectedTestCaseNames := []string{
		"verify the cluster readiness and stability",
		"verify all machines should be in Running state",
		"verify all nodes should be ready",
		"verify node count should match or exceed machine count",
		"ensure 1 worker node at least gets ready",
		"verify operator conditions authentication",
		"verify operator conditions cloud-controller-manager",
		"verify operator conditions cloud-credential",
		"verify operator conditions console",
		"verify operator conditions dns",
		"verify operator conditions etcd",
		"verify operator conditions image-registry",
		"verify operator conditions ingress",
		"verify operator conditions kube-apiserver",
		"verify operator conditions kube-controller-manager",
		"verify operator conditions kube-scheduler",
		"verify operator conditions machine-api",
		"verify operator conditions machine-config",
		"verify operator conditions monitoring",
		"verify operator conditions network",
		"verify operator conditions openshift-apiserver",
		"verify operator conditions openshift-controller-manager",
		"verify operator conditions service-ca",
		"verify operator conditions storage",
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SKIP_READINESS_CHECKS", "")
			if tc.env {
				t.Setenv("SKIP_READINESS_CHECKS", "true")
			}

			junitDir := t.TempDir()
			cmd := NewTestFailureClusterAnalysisCheckCommand()
			cmd.SetArgs(append([]string{"--junit-dir", junitDir}, tc.args...))
			require.NoError(t, cmd.Execute())

			reports, err := filepath.Glob(filepath.Join(junitDir, "junit_e2e_analysis_*.xml"))
			require.NoError(t, err)
			require.Len(t, reports, 1)

			report, err := os.ReadFile(reports[0])
			require.NoError(t, err)
			suite := &junitapi.JUnitTestSuite{}
			require.NoError(t, xml.Unmarshal(report, suite))
			require.Equal(t, expectedTestCaseNames, testCaseNames(suite.TestCases))
			require.EqualValues(t, len(expectedTestCaseNames), suite.NumTests)
			require.EqualValues(t, len(expectedTestCaseNames), suite.NumSkipped)
			for _, testCase := range suite.TestCases {
				require.NotNilf(t, testCase.SkipMessage, "test case %q was not skipped", testCase.Name)
			}
		})
	}
}

func testCaseNames(testCases []*junitapi.JUnitTestCase) []string {
	names := make([]string, 0, len(testCases))
	for _, testCase := range testCases {
		names = append(names, testCase.Name)
	}
	return names
}
