package analysis

import (
	"testing"

	osgraphtest "github.com/openshift/origin/pkg/api/graph/test"
	routeedges "github.com/openshift/origin/pkg/route/graph"
)

func TestMissingPortMapping(t *testing.T) {
	// Multiple service ports - no route port specified
	g, _, err := osgraphtest.BuildGraph("../../../api/graph/test/missing-route-port.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	routeedges.AddAllRouteEdges(g)

	markers := FindMissingPortMapping(g)
	if expected, got := 1, len(markers); expected != got {
		t.Fatalf("expected %d markers, got %d", expected, got)
	}
	if expected, got := MissingRoutePortWarning, markers[0].Key; expected != got {
		t.Fatalf("expected %s marker key, got %s", expected, got)
	}

	// Dangling route
	g, _, err = osgraphtest.BuildGraph("../../../api/graph/test/lonely-route.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	routeedges.AddAllRouteEdges(g)

	markers = FindMissingPortMapping(g)
	if expected, got := 1, len(markers); expected != got {
		t.Fatalf("expected %d markers, got %d", expected, got)
	}
	if expected, got := MissingServiceWarning, markers[0].Key; expected != got {
		t.Fatalf("expected %s marker key, got %s", expected, got)
	}
}
