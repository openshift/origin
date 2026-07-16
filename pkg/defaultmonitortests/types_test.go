package defaultmonitortests

import (
	"os"
	"testing"

	"github.com/openshift/origin/pkg/monitortestframework"
)

func TestResourceMonitorEnvVarGating(t *testing.T) {
	resourceMonitorTests := []string{
		"pod-lifecycle",
		"machine-lifecycle",
		"node-lifecycle",
		"clusteroperator-collector",
	}
	eventCollectionTests := []string{
		"event-collector",
	}

	tests := []struct {
		name                          string
		enableResourceMonitorTests    string
		enableResourceEventCollection string
		wantPresent                   []string
		wantAbsent                    []string
	}{
		{
			name:       "both env vars unset excludes resource monitors and event collection",
			wantAbsent: append(resourceMonitorTests, eventCollectionTests...),
		},
		{
			name:                       "ENABLE_RESOURCE_MONITOR_TESTS enables resource monitors only",
			enableResourceMonitorTests: "true",
			wantPresent:                resourceMonitorTests,
			wantAbsent:                 eventCollectionTests,
		},
		{
			name:                          "ENABLE_RESOURCE_EVENT_COLLECTION enables event collection only",
			enableResourceEventCollection: "true",
			wantPresent:                   eventCollectionTests,
			wantAbsent:                    resourceMonitorTests,
		},
		{
			name:                          "both env vars set enables all",
			enableResourceMonitorTests:    "true",
			enableResourceEventCollection: "true",
			wantPresent:                   append(resourceMonitorTests, eventCollectionTests...),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			os.Unsetenv(EnableResourceMonitorTestsEnv)
			os.Unsetenv(EnableResourceEventCollectionEnv)
			t.Cleanup(func() {
				os.Unsetenv(EnableResourceMonitorTestsEnv)
				os.Unsetenv(EnableResourceEventCollectionEnv)
			})

			if tc.enableResourceMonitorTests != "" {
				os.Setenv(EnableResourceMonitorTestsEnv, tc.enableResourceMonitorTests)
			}
			if tc.enableResourceEventCollection != "" {
				os.Setenv(EnableResourceEventCollectionEnv, tc.enableResourceEventCollection)
			}

			info := monitortestframework.MonitorTestInitializationInfo{
				ClusterStabilityDuringTest: monitortestframework.Stable,
			}
			registry, err := NewMonitorTestsFor(info)
			if err != nil {
				t.Fatalf("NewMonitorTestsFor returned error: %v", err)
			}

			registeredTests := registry.ListMonitorTests()
			for _, name := range tc.wantPresent {
				if !registeredTests.Has(name) {
					t.Errorf("expected monitor test %q to be registered, but it was not", name)
				}
			}
			for _, name := range tc.wantAbsent {
				if registeredTests.Has(name) {
					t.Errorf("expected monitor test %q to NOT be registered, but it was", name)
				}
			}
		})
	}
}
