package defaultmonitortests

import (
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
		setMonitorEnv                 bool
		setEventEnv                   bool
		wantPresent                   []string
		wantAbsent                    []string
	}{
		{
			name:       "both env vars unset excludes resource monitors and event collection",
			wantAbsent: append(resourceMonitorTests, eventCollectionTests...),
		},
		{
			name:                       "ENABLE_RESOURCE_MONITOR_TESTS=true enables resource monitors only",
			enableResourceMonitorTests: "true",
			setMonitorEnv:              true,
			wantPresent:                resourceMonitorTests,
			wantAbsent:                 eventCollectionTests,
		},
		{
			name:                       "ENABLE_RESOURCE_MONITOR_TESTS=1 enables resource monitors",
			enableResourceMonitorTests: "1",
			setMonitorEnv:              true,
			wantPresent:                resourceMonitorTests,
			wantAbsent:                 eventCollectionTests,
		},
		{
			name:                       "ENABLE_RESOURCE_MONITOR_TESTS=0 does not enable resource monitors",
			enableResourceMonitorTests: "0",
			setMonitorEnv:              true,
			wantAbsent:                 append(resourceMonitorTests, eventCollectionTests...),
		},
		{
			name:                       "ENABLE_RESOURCE_MONITOR_TESTS=FALSE does not enable resource monitors",
			enableResourceMonitorTests: "FALSE",
			setMonitorEnv:              true,
			wantAbsent:                 append(resourceMonitorTests, eventCollectionTests...),
		},
		{
			name:                          "ENABLE_RESOURCE_EVENT_COLLECTION=true enables event collection only",
			enableResourceEventCollection: "true",
			setEventEnv:                   true,
			wantPresent:                   eventCollectionTests,
			wantAbsent:                    resourceMonitorTests,
		},
		{
			name:                          "ENABLE_RESOURCE_EVENT_COLLECTION=0 does not enable event collection",
			enableResourceEventCollection: "0",
			setEventEnv:                   true,
			wantAbsent:                    append(resourceMonitorTests, eventCollectionTests...),
		},
		{
			name:                          "both env vars set to true enables all",
			enableResourceMonitorTests:    "true",
			enableResourceEventCollection: "true",
			setMonitorEnv:                 true,
			setEventEnv:                   true,
			wantPresent:                   append(resourceMonitorTests, eventCollectionTests...),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setMonitorEnv {
				t.Setenv(EnableResourceMonitorTestsEnv, tc.enableResourceMonitorTests)
			}
			if tc.setEventEnv {
				t.Setenv(EnableResourceEventCollectionEnv, tc.enableResourceEventCollection)
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
