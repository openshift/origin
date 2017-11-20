package clientcmd

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/sets"
)

// TestRunGenerators makes sure we catch new generators added to `oc run`
func TestRunGenerators(t *testing.T) {
	f := NewFactory(nil)

	// Contains the run generators we expect to see
	expectedRunGenerators := sets.NewString(
		// kube generators
		"run/v1",
		"run-pod/v1",
		"deployment/apps.v1beta1",
		"deployment/v1beta1",
		"job/v1",
		"cronjob/v2alpha1",
		"cronjob/v1beta1",

		// origin generators
		"run-controller/v1", // legacy alias for run/v1
		"deploymentconfig/v1",
	).List()

	runGenerators := sets.StringKeySet(f.Generators("run")).List()
	if !reflect.DeepEqual(expectedRunGenerators, runGenerators) {
		t.Errorf("Expected run generators:%#v, got:\n%#v", expectedRunGenerators, runGenerators)
	}
}

func TestComputeDiscoverCacheDir(t *testing.T) {
	testCases := []struct {
		name      string
		parentDir string
		host      string

		expected string
	}{
		{
			name:      "simple append",
			parentDir: "~/",
			host:      "localhost:8443",
			expected:  "~/localhost_8443",
		},
		{
			name:      "with path",
			parentDir: "~/",
			host:      "localhost:8443/prefix",
			expected:  "~/localhost_8443/prefix",
		},
		{
			name:      "dotted name",
			parentDir: "~/",
			host:      "mine.example.org:8443",
			expected:  "~/mine.example.org_8443",
		},
		{
			name:      "IP",
			parentDir: "~/",
			host:      "127.0.0.1:8443",
			expected:  "~/127.0.0.1_8443",
		},
		{
			// restricted characters from: https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx#naming_conventions
			// it's not a complete list because they have a very helpful: "Any other character that the target file system does not allow."
			name:      "windows safe",
			parentDir: "~/",
			host:      `<>:"\|?*`,
			expected:  "~/________",
		},
	}

	for _, tc := range testCases {
		actual := computeDiscoverCacheDir(tc.parentDir, tc.host)
		if actual != tc.expected {
			t.Errorf("%s: expected %v, got %v", tc.name, tc.expected, actual)
		}
	}
}
