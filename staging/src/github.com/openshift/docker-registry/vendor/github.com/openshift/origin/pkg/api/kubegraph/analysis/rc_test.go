package analysis

import (
	"testing"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	osgraphtest "github.com/openshift/origin/pkg/api/graph/test"
	kubeedges "github.com/openshift/origin/pkg/api/kubegraph"
)

func TestDuelingRC(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../api/graph/test/dueling-rcs.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kubeedges.AddAllManagedByControllerPodEdges(g)

	markers := FindDuelingReplicationControllers(g, osgraph.DefaultNamer)
	if e, a := 2, len(markers); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}

	expectedRC1 := g.Find(osgraph.UniqueName("ReplicationController|/rc-1"))
	expectedRC2 := g.Find(osgraph.UniqueName("ReplicationController|/rc-2"))
	found1 := false
	found2 := false

	for i := 0; i < 2; i++ {
		actualPod := osgraph.GetTopLevelContainerNode(g, markers[i].RelatedNodes[0])
		expectedPod := g.Find(osgraph.UniqueName("Pod|/conflicted-pod"))
		if e, a := expectedPod.ID(), actualPod.ID(); e != a {
			t.Errorf("expected %v, got %v", e, a)
		}

		actualOtherRC := osgraph.GetTopLevelContainerNode(g, markers[i].RelatedNodes[1])

		actualRC := markers[i].Node
		if e, a := expectedRC1.ID(), actualRC.ID(); e == a {
			found1 = true

			expectedOtherRC := expectedRC2
			if e, a := expectedOtherRC.ID(), actualOtherRC.ID(); e != a {
				t.Errorf("expected %v, got %v", e, a)
			}
		}
		if e, a := expectedRC2.ID(), actualRC.ID(); e == a {
			found2 = true

			expectedOtherRC := expectedRC1
			if e, a := expectedOtherRC.ID(), actualOtherRC.ID(); e != a {
				t.Errorf("expected %v, got %v", e, a)
			}
		}
	}

	if !found1 {
		t.Errorf("expected %v, got %v", expectedRC1, markers)
	}

	if !found2 {
		t.Errorf("expected %v, got %v", expectedRC2, markers)
	}
}
