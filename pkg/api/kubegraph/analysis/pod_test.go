package analysis

import (
	"testing"

	osgraphtest "github.com/openshift/origin/pkg/api/graph/test"
)

func TestRestartingPodWarning(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../api/graph/test/restarting-pod.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	markers := FindRestartingPods(g)
	if e, a := 1, len(markers); e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}
	if e, a := RestartingPodWarning, markers[0].Key; e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}
}
