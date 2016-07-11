package analysis

import (
	"strings"
	"testing"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	osgraphtest "github.com/openshift/origin/pkg/api/graph/test"
	"github.com/openshift/origin/pkg/api/kubegraph"
	deploygraph "github.com/openshift/origin/pkg/deploy/graph"
)

func TestHPAMissingCPUTargetError(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("./../../../api/graph/test/hpa-missing-cpu-target.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	markers := FindHPASpecsMissingCPUTargets(g, osgraph.DefaultNamer)
	if len(markers) != 1 {
		t.Fatalf("expected to find one HPA spec missing a CPU target, got %d", len(markers))
	}

	if actual, expected := markers[0].Severity, osgraph.ErrorSeverity; actual != expected {
		t.Errorf("expected HPA missing CPU target to be %v, got %v", expected, actual)
	}

	if actual, expected := markers[0].Key, HPAMissingCPUTargetError; actual != expected {
		t.Errorf("expected marker type %v, got %v", expected, actual)
	}

	patchString := `-p '{"spec":{"targetCPUUtilizationPercentage": 80}}'`
	if !strings.HasSuffix(string(markers[0].Suggestion), patchString) {
		t.Errorf("expected suggestion to end with patch JSON path, got %q", markers[0].Suggestion)
	}
}

func TestHPAMissingScaleRefError(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("./../../../api/graph/test/hpa-missing-scale-ref.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	markers := FindHPASpecsMissingScaleRefs(g, osgraph.DefaultNamer)
	if len(markers) != 1 {
		t.Fatalf("expected to find one HPA spec missing a scale ref, got %d", len(markers))
	}

	if actual, expected := markers[0].Severity, osgraph.ErrorSeverity; actual != expected {
		t.Errorf("expected HPA missing scale ref to be %v, got %v", expected, actual)
	}

	if actual, expected := markers[0].Key, HPAMissingScaleRefError; actual != expected {
		t.Errorf("expected marker type %v, got %v", expected, actual)
	}
}

func TestOverlappingHPAsWarning(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("./../../../api/graph/test/overlapping-hpas.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kubegraph.AddHPAScaleRefEdges(g)
	deploygraph.AddAllDeploymentEdges(g)

	markers := FindOverlappingHPAs(g, osgraph.DefaultNamer)
	if len(markers) != 8 {
		t.Fatalf("expected to find eight overlapping HPA markers, got %d", len(markers))
	}

	for _, marker := range markers {
		if actual, expected := marker.Severity, osgraph.WarningSeverity; actual != expected {
			t.Errorf("expected overlapping HPAs to be %v, got %v", expected, actual)
		}

		if actual, expected := marker.Key, HPAOverlappingScaleRefWarning; actual != expected {
			t.Errorf("expected marker type %v, got %v", expected, actual)
		}
	}
}
