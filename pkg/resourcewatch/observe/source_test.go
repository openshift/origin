package observe

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestResourcesToWatch(t *testing.T) {
	t.Parallel()

	eventsGVR := schema.GroupVersionResource{Group: "events.k8s.io", Version: "v1", Resource: "events"}

	tests := []struct {
		name                   string
		monitorEnabled         bool
		eventCollectionEnabled bool
		wantEmpty              bool
		wantEvents             bool
		wantMonitor            bool
	}{
		{
			name:      "both disabled returns no resources",
			wantEmpty: true,
		},
		{
			name:           "only monitor enabled returns monitor resources without events",
			monitorEnabled: true,
			wantMonitor:    true,
		},
		{
			name:                   "only event collection enabled returns only events",
			eventCollectionEnabled: true,
			wantEvents:             true,
		},
		{
			name:                   "both enabled returns all resources including events",
			monitorEnabled:         true,
			eventCollectionEnabled: true,
			wantMonitor:            true,
			wantEvents:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resources := resourcesToWatch(tt.monitorEnabled, tt.eventCollectionEnabled)

			if tt.wantEmpty {
				if len(resources) != 0 {
					t.Fatalf("expected no resources, got %d", len(resources))
				}
				return
			}

			hasEvents := containsGVR(resources, eventsGVR)
			if tt.wantEvents && !hasEvents {
				t.Fatal("expected events GVR to be present")
			}
			if !tt.wantEvents && hasEvents {
				t.Fatal("expected events GVR to be absent")
			}

			hasMonitor := len(resources) > 1 || (len(resources) == 1 && !hasEvents)
			if tt.wantMonitor && !hasMonitor {
				t.Fatal("expected monitor resources to be present")
			}
			if !tt.wantMonitor && hasMonitor {
				t.Fatal("expected only event resources")
			}
		})
	}
}

func TestMonitorResourcesDoNotIncludeEvents(t *testing.T) {
	t.Parallel()

	eventsGVR := schema.GroupVersionResource{Group: "events.k8s.io", Version: "v1", Resource: "events"}
	resources := monitorResources()

	if len(resources) == 0 {
		t.Fatal("expected monitor resources to be non-empty")
	}
	if containsGVR(resources, eventsGVR) {
		t.Fatal("monitor resources should not include events")
	}
}

func TestEventResourcesContainOnlyEvents(t *testing.T) {
	t.Parallel()

	wantEvents := schema.GroupVersionResource{Group: "events.k8s.io", Version: "v1", Resource: "events"}
	resources := eventResources()
	if len(resources) != 1 || resources[0] != wantEvents {
		t.Fatalf("unexpected event resources: expected %#v, got %#v", wantEvents, resources)
	}
}

func containsGVR(resources []schema.GroupVersionResource, target schema.GroupVersionResource) bool {
	for _, r := range resources {
		if r == target {
			return true
		}
	}
	return false
}
