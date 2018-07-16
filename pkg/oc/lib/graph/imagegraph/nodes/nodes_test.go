package nodes

import (
	"testing"

	"github.com/openshift/origin/pkg/oc/cli/admin/prune/imageprune/testutil"
	"github.com/openshift/origin/pkg/oc/lib/graph/genericgraph"
)

const registryHost = "registry.io"

func TestEnsureImageNode(t *testing.T) {
	g := genericgraph.New()

	img1 := testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000001", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000001")
	img2 := testutil.Image("sha256:0000000000000000000000000000000000000000000000000000000000000002", registryHost+"/foo/bar@sha256:0000000000000000000000000000000000000000000000000000000000000002")
	img3 := testutil.Image("sha356:0000000000000000000000000000000000000000000000000000000000000003", registryHost+"/foo/bar@sha356:0000000000000000000000000000000000000000000000000000000000000003")

	node := FindImage(g, img1.Name)
	if node != nil {
		t.Fatalf("got unexpected image")
	}
	n1 := EnsureImageNode(g, &img1)
	if n1 == nil {
		t.Fatalf("failed to create node")
	}
	node = FindImage(g, img1.Name)
	if node == nil {
		t.Fatalf("expected node, not nil")
	}
	if node.Image != &img1 {
		t.Fatalf("got unexpected image: %s != %s", node.Image.Name, img1.Name)
	}

	n2 := EnsureImageNode(g, &img2)
	if n2 == nil {
		t.Fatalf("expected node, not nil")
	}
	if len(g.Nodes()) != 2 {
		t.Fatalf("unexpected number of nodes: %d != %d", len(g.Nodes()), 2)
	}

	g.RemoveNode(n1)

	// removing node deletes its edges but the image is still known
	node = FindImage(g, img1.Name)
	if node == nil {
		t.Fatalf("removed node can still be found")
	}
	// but it's no longer iterable
	if len(g.Nodes()) != 1 {
		t.Fatalf("unexpected number of nodes: %d != %d", len(g.Nodes()), 1)
	}

	// add the image1 again
	n1 = EnsureImageNode(g, &img1)
	if n1 == nil {
		t.Fatalf("failed to create node")
	}
	node = FindImage(g, img1.Name)
	if node == nil {
		t.Fatalf("expected node, not nil")
	}
	if node.Image != &img1 {
		t.Fatalf("got unexpected image: %s != %s", node.Image.Name, img1.Name)
	}
	/**
	 * NOTE: it looks like removed node cannot be re-added
	if len(g.Nodes()) != 2 {
		t.Fatalf("unexpected number of nodes: %d != %d", len(g.Nodes()), 2)
	}
	*/

	// re-add the image1 once again
	tmp := EnsureImageNode(g, &img1)
	if tmp == nil {
		t.Fatalf("failed to find the node")
	}
	if tmp != n1 {
		t.Fatalf("got unexpected node: %v != %v", node, n1)
	}

	g.RemoveNode(n2)
	node = FindImage(g, img2.Name)
	if node == nil {
		t.Fatalf("removed node can still be found")
	}
	if len(g.Nodes()) != 0 {
		t.Fatalf("unexpected number of nodes: %d != %d", len(g.Nodes()), 0)
	}

	n3 := EnsureImageNode(g, &img3)
	if n3 == nil {
		t.Fatalf("failed to create node")
	}
	node = FindImage(g, img3.Name)
	if node == nil {
		t.Fatalf("expected node, not nil")
	}
	if node.Image != &img3 {
		t.Fatalf("got unexpected image: %s != %s", node.Image.Name, img3.Name)
	}
	if len(g.Nodes()) != 1 {
		t.Fatalf("unexpected number of nodes: %d != %d", len(g.Nodes()), 1)
	}
}
