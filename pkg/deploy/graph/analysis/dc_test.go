package analysis

import (
	"testing"

	osgraphtest "github.com/openshift/origin/pkg/api/graph/test"
	kubeedges "github.com/openshift/origin/pkg/api/kubegraph"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploygraph "github.com/openshift/origin/pkg/deploy/graph/nodes"
)

func TestCheckMountedSecrets(t *testing.T) {
	g, objs, err := osgraphtest.BuildGraph("../../../api/graph/test/bad_secret_refs.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var dc *deployapi.DeploymentConfig
	for _, obj := range objs {
		if currDC, ok := obj.(*deployapi.DeploymentConfig); ok {
			if dc != nil {
				t.Errorf("got more than one dc: %v", currDC)
			}
			dc = currDC
		}
	}

	kubeedges.AddAllRequestedServiceAccountEdges(g)
	kubeedges.AddAllMountableSecretEdges(g)
	kubeedges.AddAllMountedSecretEdges(g)

	dcNode := g.Find(deploygraph.DeploymentConfigNodeName(dc))
	unmountable, missing := CheckMountedSecrets(g, dcNode.(*deploygraph.DeploymentConfigNode))

	if e, a := 2, len(unmountable); e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}

	if e, a := 1, len(missing); e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}
	if e, a := "missing-secret", missing[0].Name; e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}
}
