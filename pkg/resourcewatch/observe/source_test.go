package observe

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestResourcesToWatch(t *testing.T) {
	t.Parallel()

	eventsGVR := schema.GroupVersionResource{Group: "events.k8s.io", Version: "v1", Resource: "events"}

	tests := []struct {
		name         string
		enableEvents bool
		wantEvents   bool
	}{
		{
			name:       "events disabled excludes events GVR",
			wantEvents: false,
		},
		{
			name:         "events enabled includes events GVR",
			enableEvents: true,
			wantEvents:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resources := resourcesToWatch(tt.enableEvents)

			if len(resources) == 0 {
				t.Fatal("expected resources to be non-empty")
			}

			hasEvents := containsGVR(resources, eventsGVR)
			if tt.wantEvents && !hasEvents {
				t.Fatal("expected events GVR to be present")
			}
			if !tt.wantEvents && hasEvents {
				t.Fatal("expected events GVR to be absent")
			}
		})
	}
}

func TestResourcesToWatchAlwaysIncludesBaseResources(t *testing.T) {
	t.Parallel()

	withoutEvents := resourcesToWatch(false)
	withEvents := resourcesToWatch(true)

	if len(withEvents) != len(withoutEvents)+1 {
		t.Fatalf("enabling events should add exactly 1 resource, got %d without and %d with",
			len(withoutEvents), len(withEvents))
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
