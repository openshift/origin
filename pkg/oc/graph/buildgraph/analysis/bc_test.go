package analysis

import (
	"strings"
	"testing"

	buildedges "github.com/openshift/origin/pkg/oc/graph/buildgraph"
	buildgraph "github.com/openshift/origin/pkg/oc/graph/buildgraph/nodes"
	osgraph "github.com/openshift/origin/pkg/oc/graph/genericgraph"
	osgraphtest "github.com/openshift/origin/pkg/oc/graph/genericgraph/test"
	imageedges "github.com/openshift/origin/pkg/oc/graph/imagegraph"
	imagegraph "github.com/openshift/origin/pkg/oc/graph/imagegraph/nodes"
)

func TestUnpushableBuild(t *testing.T) {
	// Unconfigured internal registry
	g, _, err := osgraphtest.BuildGraph("../../../graph/genericgraph/test/unpushable-build.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	buildedges.AddAllInputOutputEdges(g)
	imageedges.AddAllImageStreamRefEdges(g)
	imageedges.AddAllImageStreamImageRefEdges(g)

	markers := FindUnpushableBuildConfigs(g, osgraph.DefaultNamer)
	if e, a := 2, len(markers); e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}

	if got, expected := markers[0].Key, MissingRequiredRegistryErr; got != expected {
		t.Fatalf("expected marker key %q, got %q", expected, got)
	}

	actualBC := osgraph.GetTopLevelContainerNode(g, markers[0].Node)
	expectedBC1 := g.Find(osgraph.UniqueName("BuildConfig|example/ruby-hello-world"))
	expectedBC2 := g.Find(osgraph.UniqueName("BuildConfig|example/ruby-hello-world-2"))
	if e1, e2, a := expectedBC1.ID(), expectedBC2.ID(), actualBC.ID(); e1 != a && e2 != a {
		t.Errorf("expected either %v or %v, got %v", e1, e2, a)
	}

	actualIST := markers[0].RelatedNodes[0]
	expectedIST := g.Find(osgraph.UniqueName("ImageStreamTag|example/ruby-hello-world:latest"))
	if e, a := expectedIST.ID(), actualIST.ID(); e != a {
		t.Errorf("expected %v, got %v: \n%v", e, a, g)
	}

	// Missing image stream
	g, _, err = osgraphtest.BuildGraph("../../../graph/genericgraph/test/unpushable-build-2.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	buildedges.AddAllInputOutputEdges(g)
	imageedges.AddAllImageStreamRefEdges(g)
	imageedges.AddAllImageStreamImageRefEdges(g)

	markers = FindUnpushableBuildConfigs(g, osgraph.DefaultNamer)
	if e, a := 1, len(markers); e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}

	if got, expected := markers[0].Key, MissingOutputImageStreamErr; got != expected {
		t.Fatalf("expected marker key %q, got %q", expected, got)
	}
}

func TestPushableBuild(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../graph/genericgraph/test/pushable-build.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	buildedges.AddAllInputOutputEdges(g)
	imageedges.AddAllImageStreamRefEdges(g)
	imageedges.AddAllImageStreamImageRefEdges(g)

	if e, a := 0, len(FindUnpushableBuildConfigs(g, osgraph.DefaultNamer)); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
}

func TestImageStreamPresent(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../graph/genericgraph/test/prereq-image-present.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	buildedges.AddAllInputOutputEdges(g)
	imageedges.AddAllImageStreamRefEdges(g)
	imageedges.AddAllImageStreamImageRefEdges(g)

	if e, a := 0, len(FindMissingInputImageStreams(g, osgraph.DefaultNamer)); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
}

func TestImageStreamTagMissing(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../graph/genericgraph/test/prereq-image-present-notag.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	buildedges.AddAllInputOutputEdges(g)
	imageedges.AddAllImageStreamRefEdges(g)
	imageedges.AddAllImageStreamImageRefEdges(g)

	markers := FindMissingInputImageStreams(g, osgraph.DefaultNamer)
	if e, a := 4, len(markers); e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}

	var actualImportOrBuild, actualImportOnly, actualSpecificHex int
	expectedImportOrBuild := 2
	expectedImportOnly := 1
	expectedSpecificHex := 1
	for _, marker := range markers {
		if got, expected1, expected2 := marker.Key, MissingImageStreamImageWarning, MissingImageStreamTagWarning; got != expected1 && got != expected2 {
			t.Fatalf("expected marker key %q or %q, got %q", expected1, expected2, got)
		} else {
			if strings.Contains(marker.Suggestion.String(), "oc start-build") {
				actualImportOrBuild++
			}
			if strings.Contains(marker.Suggestion.String(), "needs to be imported.") {
				actualImportOnly++
			}
			if strings.Contains(marker.Suggestion.String(), "hexadecimal ID") {
				actualSpecificHex++
			}
		}
	}
	if actualImportOnly != expectedImportOnly {
		t.Fatalf("expected %d import only suggestions but got %d", expectedImportOnly, actualImportOnly)
	}
	if actualImportOrBuild != expectedImportOrBuild {
		t.Fatalf("expected %d import or build suggestions but got %d", expectedImportOrBuild, actualImportOrBuild)
	}
	if actualSpecificHex != expectedSpecificHex {
		t.Fatalf("expected %d import specific image suggestions but got %d", expectedSpecificHex, actualSpecificHex)
	}
}

func TestImageStreamMissing(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../graph/genericgraph/test/prereq-image-not-present.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	buildedges.AddAllInputOutputEdges(g)
	imageedges.AddAllImageStreamRefEdges(g)
	imageedges.AddAllImageStreamImageRefEdges(g)

	markers := FindMissingInputImageStreams(g, osgraph.DefaultNamer)
	if e, a := 3, len(markers); e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}

}

func TestBuildConfigNoOutput(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../graph/genericgraph/test/bc-missing-output.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// we were getting go panics with nil refs cause output destinations are not required for BuildConfigs
	buildedges.AddAllInputOutputEdges(g)
}

func TestCircularDeps(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../graph/genericgraph/test/circular.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	buildedges.AddAllInputOutputEdges(g)

	if len(FindCircularBuilds(g, osgraph.DefaultNamer)) != 1 {
		t.Fatalf("expected having circular dependencies")
	}

	not, _, err := osgraphtest.BuildGraph("../../../graph/genericgraph/test/circular-not.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	buildedges.AddAllInputOutputEdges(not)

	if len(FindCircularBuilds(not, osgraph.DefaultNamer)) != 0 {
		t.Fatalf("expected not having circular dependencies")
	}
}

func TestPendingImageStreamTag(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../graph/genericgraph/test/unpushable-build.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	buildedges.AddAllInputOutputEdges(g)
	buildedges.AddAllBuildEdges(g)
	imageedges.AddAllImageStreamRefEdges(g)
	imageedges.AddAllImageStreamImageRefEdges(g)

	// Drop the build to showcase a TagNotAvailable warning (should happen when no
	// build is new, pending, or running currently)
	nodeFn := osgraph.NodesOfKind(imagegraph.ImageStreamTagNodeKind, buildgraph.BuildConfigNodeKind)
	edgeFn := osgraph.EdgesOfKind(buildedges.BuildInputImageEdgeKind, buildedges.BuildOutputEdgeKind)
	g = g.Subgraph(nodeFn, edgeFn)

	markers := FindPendingTags(g, osgraph.DefaultNamer)
	if e, a := 1, len(markers); e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}

	if got, expected := markers[0].Key, TagNotAvailableWarning; got != expected {
		t.Fatalf("expected marker key %q, got %q", expected, got)
	}
}

func TestLatestBuildFailed(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../graph/genericgraph/test/failed-build.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	buildedges.AddAllInputOutputEdges(g)
	buildedges.AddAllBuildEdges(g)
	imageedges.AddAllImageStreamRefEdges(g)
	imageedges.AddAllImageStreamImageRefEdges(g)

	markers := FindPendingTags(g, osgraph.DefaultNamer)
	if e, a := 1, len(markers); e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}

	if got, expected := markers[0].Key, LatestBuildFailedErr; got != expected {
		t.Fatalf("expected marker key %q, got %q", expected, got)
	}
	if !strings.Contains(markers[0].Suggestion.String(), "oc logs -f bc/ruby-hello-world") {
		t.Fatalf("expected oc logs -f bc/ruby-hello-world, got %s", markers[0].Suggestion.String())
	}
}
