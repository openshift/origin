package analysis

import (
	"testing"

	osgraphtest "github.com/openshift/origin/pkg/api/graph/test"
	buildedges "github.com/openshift/origin/pkg/build/graph"
	deployedges "github.com/openshift/origin/pkg/deploy/graph"
	imageedges "github.com/openshift/origin/pkg/image/graph"
)

func TestMissingImageStreamTag(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../api/graph/test/missing-istag.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	buildedges.AddAllInputOutputEdges(g)
	deployedges.AddAllTriggerEdges(g)
	imageedges.AddAllImageStreamRefEdges(g)

	markers := FindDeploymentConfigTriggerErrors(g)
	if e, a := 1, len(markers); e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}

	if got, expected := markers[0].Key, MissingImageStreamTagWarning; got != expected {
		t.Fatalf("expected marker key %q, got %q", expected, got)
	}
}

func TestMissingImageStream(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../api/graph/test/unpushable-build-2.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	buildedges.AddAllInputOutputEdges(g)
	deployedges.AddAllTriggerEdges(g)
	imageedges.AddAllImageStreamRefEdges(g)

	markers := FindDeploymentConfigTriggerErrors(g)
	if e, a := 1, len(markers); e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}

	if got, expected := markers[0].Key, MissingImageStreamErr; got != expected {
		t.Fatalf("expected marker key %q, got %q", expected, got)
	}
}
