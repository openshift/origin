package analysis

import (
	"testing"

	osgraph "github.com/openshift/origin/pkg/api/graph"
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
	imageedges.AddAllImageStreamImageRefEdges(g)

	markers := FindDeploymentConfigTriggerErrors(g, osgraph.DefaultNamer)
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
	imageedges.AddAllImageStreamImageRefEdges(g)

	markers := FindDeploymentConfigTriggerErrors(g, osgraph.DefaultNamer)
	if e, a := 1, len(markers); e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}

	if got, expected := markers[0].Key, MissingImageStreamErr; got != expected {
		t.Fatalf("expected marker key %q, got %q", expected, got)
	}
}

func TestMissingReadinessProbe(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../api/graph/test/unpushable-build-2.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	buildedges.AddAllInputOutputEdges(g)
	deployedges.AddAllTriggerEdges(g)
	imageedges.AddAllImageStreamRefEdges(g)

	markers := FindDeploymentConfigReadinessWarnings(g, osgraph.DefaultNamer, "command probe")
	if e, a := 1, len(markers); e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}

	if got, expected := markers[0].Key, MissingReadinessProbeWarning; got != expected {
		t.Fatalf("expected marker key %q, got %q", expected, got)
	}
}

func TestSingleHostVolumeError(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../api/graph/test/dc-with-claim.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	deployedges.AddAllVolumeClaimEdges(g)

	markers := FindPersistentVolumeClaimWarnings(g, osgraph.DefaultNamer)
	if e, a := 1, len(markers); e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}

	if got, expected := markers[0].Key, SingleHostVolumeWarning; got != expected {
		t.Fatalf("expected marker key %q, got %q", expected, got)
	}
}
