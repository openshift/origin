package analysis

import (
	"testing"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	osgraphtest "github.com/openshift/origin/pkg/api/graph/test"
	kubeedges "github.com/openshift/origin/pkg/api/kubegraph"
)

func TestMissingSecrets(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../api/graph/test/bad_secret_refs.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kubeedges.AddAllRequestedServiceAccountEdges(g)
	kubeedges.AddAllMountableSecretEdges(g)
	kubeedges.AddAllMountedSecretEdges(g)

	markers := FindMissingSecrets(g, osgraph.DefaultNamer)
	if e, a := 1, len(markers); e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}

	actualDC := osgraph.GetTopLevelContainerNode(g, markers[0].Node)
	expectedDC := g.Find(osgraph.UniqueName("DeploymentConfig|/docker-nfs-server"))
	if e, a := expectedDC.ID(), actualDC.ID(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}

	actualSecret := markers[0].RelatedNodes[0]
	expectedSecret := g.Find(osgraph.UniqueName("Secret|/missing-secret"))
	if e, a := expectedSecret.ID(), actualSecret.ID(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
}

func TestUnmountableSecrets(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../api/graph/test/bad_secret_refs.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kubeedges.AddAllRequestedServiceAccountEdges(g)
	kubeedges.AddAllMountableSecretEdges(g)
	kubeedges.AddAllMountedSecretEdges(g)

	markers := FindUnmountableSecrets(g, osgraph.DefaultNamer)
	if e, a := 2, len(markers); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}

	expectedSecret1 := g.Find(osgraph.UniqueName("Secret|/missing-secret"))
	expectedSecret2 := g.Find(osgraph.UniqueName("Secret|/unmountable-secret"))
	found1 := false
	found2 := false

	for i := 0; i < 2; i++ {
		actualDC := osgraph.GetTopLevelContainerNode(g, markers[i].Node)
		expectedDC := g.Find(osgraph.UniqueName("DeploymentConfig|/docker-nfs-server"))
		if e, a := expectedDC.ID(), actualDC.ID(); e != a {
			t.Errorf("expected %v, got %v", e, a)
		}

		actualSecret := markers[i].RelatedNodes[0]
		if e, a := expectedSecret1.ID(), actualSecret.ID(); e == a {
			found1 = true
		}
		if e, a := expectedSecret2.ID(), actualSecret.ID(); e == a {
			found2 = true
		}
	}

	if !found1 {
		t.Errorf("expected %v, got %v", expectedSecret1, markers)
	}

	if !found2 {
		t.Errorf("expected %v, got %v", expectedSecret2, markers)
	}
}

func TestMissingLivenessProbes(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../api/graph/test/simple-deployment.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kubeedges.AddAllExposedPodEdges(g)

	markers := FindMissingLivenessProbes(g, osgraph.DefaultNamer, "oc set probe")
	if e, a := 1, len(markers); e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}

	actualDC := osgraph.GetTopLevelContainerNode(g, markers[0].Node)
	expectedDC := g.Find(osgraph.UniqueName("DeploymentConfig|/simple-deployment"))
	if e, a := expectedDC.ID(), actualDC.ID(); e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
}
