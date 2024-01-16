package certgraph

import (
	"fmt"
	"os"
	"strings"

	"github.com/gonum/graph"
	"github.com/gonum/graph/encoding/dot"
	"github.com/gonum/graph/simple"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"k8s.io/apimachinery/pkg/util/sets"
)

type resourcesImpl struct {
	resources []Resource
}

func (r *resourcesImpl) Add(resource Resource) {
	r.resources = append(r.resources, resource)
}

func (r *resourcesImpl) AllResources() []Resource {
	ret := []Resource{}
	for _, v := range r.resources {
		ret = append(ret, v)
	}
	return ret
}

func (r *resourcesImpl) Roots() []Resource {
	ret := []Resource{}
	for _, resource := range r.AllResources() {
		if len(resource.SourceNames()) > 0 {
			continue
		}
		ret = append(ret, resource)
	}
	return ret
}

type resourceGraphNode struct {
	simple.Node
	Resource Resource
}

var _ CertGraphNode = resourceGraphNode{}

func (n resourceGraphNode) Name() string {
	return n.Resource.Name()
}

func (n resourceGraphNode) GetCABundle() *certgraphapi.CertificateAuthorityBundle {
	return n.Resource.GetCABundle()
}

func (n resourceGraphNode) GetCertKeyPair() *certgraphapi.CertKeyPair {
	return n.Resource.GetCertKeyPair()
}

// DOTAttributes implements an attribute getter for the DOT encoding
func (n resourceGraphNode) DOTAttributes() []dot.Attribute {
	certCoordinates := n.GetCertKeyPair()
	caBundleCoordinates := n.GetCABundle()

	color := "white"
	if certCoordinates != nil {
		switch {
		case certCoordinates.Spec.Details.CertType == "SignerCertDetails":
			color = `"#c7bfff"` // purple
		case certCoordinates.Spec.Details.CertType == "ServingCertDetails":
			color = `"#bdebfd"` // blue
		case certCoordinates.Spec.Details.CertType == "ClientCertDetails":
			color = `"#c8fbcd"` // green
		case certCoordinates.Spec.Details.CertType == "Multiple":
			color = `"#fffdb8"` // yellow
		}
	}
	if caBundleCoordinates != nil {
		color = `"#fda172"` // orange
	}

	locations := []string{}
	if certCoordinates != nil {
		for _, curr := range certCoordinates.Spec.SecretLocations {
			locations = append(locations, fmt.Sprintf("secret/%s -n%s", curr.Name, curr.Namespace))
		}
		for _, curr := range certCoordinates.Spec.OnDiskLocations {
			locations = append(locations, fmt.Sprintf("file://%s,file://%s", curr.Cert.Path, curr.Key.Path))
		}
	}
	if caBundleCoordinates != nil {
		for _, curr := range caBundleCoordinates.Spec.ConfigMapLocations {
			locations = append(locations, fmt.Sprintf("configmaps/%s -n%s", curr.Name, curr.Namespace))
		}
		for _, curr := range caBundleCoordinates.Spec.OnDiskLocations {
			locations = append(locations, fmt.Sprintf("file://%s", curr.Path))
		}
	}

	nodeName := ""

	if certCoordinates != nil {
		if strings.HasPrefix(n.Name(), "certificate-no-key/") {
			nodeName = n.Name()
		} else {
			if len(certCoordinates.LogicalName) > 0 {
				nodeName = "certkeypair/" + certCoordinates.LogicalName
			} else {
				nodeName = "certkeypair/" + certCoordinates.Name
			}
		}
	}
	if caBundleCoordinates != nil {
		if len(caBundleCoordinates.LogicalName) > 0 {
			nodeName = "cabundle/" + caBundleCoordinates.LogicalName
		} else {
			nodeName = "cabundle/" + caBundleCoordinates.Name
		}
	}

	label := fmt.Sprintf("%s\n\n%s\n%s",
		nodeName,
		strings.Join(locations, "\n    "),
		n.Resource.GetNote())

	return []dot.Attribute{
		{Key: "label", Value: fmt.Sprintf("%q", label)},
		{Key: "style", Value: "filled"},
		{Key: "fillcolor", Value: color},
	}
}

func (r *resourcesImpl) NewGraph() (graph.Directed, error) {
	g := simple.NewDirectedGraph(1.0, 0.0)

	typeAndCommonNameToNode := map[string]graph.Node{}
	allResourceIndexToNode := map[int]graph.Node{}

	// make all nodes
	allResources := r.AllResources()
	for i := range allResources {
		resource := allResources[i]
		id := g.NewNodeID()
		node := resourceGraphNode{Node: simple.Node(id), Resource: resource}

		typeAndCommonName := resource.Name()

		typeAndCommonNameToNode[typeAndCommonName] = node
		allResourceIndexToNode[i] = node
		g.AddNode(node)
	}

	// find all nodes to things that don't exist
	nodesThatDoNotExistYet := sets.NewString()
	for i := range allResources {
		resource := allResources[i]
		for _, sourceName := range resource.SourceNames() {
			_, ok := typeAndCommonNameToNode[sourceName]
			if !ok {
				nodesThatDoNotExistYet.Insert(sourceName)
			}
		}
	}

	// make all nodes that don't exist yet
	for _, nodeName := range nodesThatDoNotExistYet.List() {
		if !strings.HasPrefix(nodeName, "cert/") {
			continue
		}
		if CertNamesToHide.Has(nodeName[len("cert/"):]) {
			continue
		}

		id := g.NewNodeID()
		node := resourceGraphNode{Node: simple.Node(id), Resource: newVirtualCertSource(nodeName)}
		typeAndCommonNameToNode[nodeName] = node
		g.AddNode(node)
	}

	// make all edges
	for i := range allResources {
		resource := allResources[i]
		for _, sourceName := range resource.SourceNames() {
			from, ok := typeAndCommonNameToNode[sourceName]
			if !ok {
				//fmt.Fprintf(os.Stderr, "missing from: %q\n", sourceName)
				continue
			}

			to, ok := allResourceIndexToNode[i]
			if !ok {
				fmt.Fprintf(os.Stderr, "missing to: %q\n", sourceName)
				continue
			}
			if from.ID() == to.ID() {
				//fmt.Fprintf(os.Stderr, "adding self-edge %T %q\n", resource, resource.String())
				continue
			}

			g.SetEdge(simple.Edge{F: from, T: to})
		}
	}

	return g, nil
}

// Quote takes an arbitrary DOT ID and escapes any quotes that is contains.
// The resulting string is quoted again to guarantee that it is a valid ID.
// DOT graph IDs can be any double-quoted string
// See http://www.graphviz.org/doc/info/lang.html
func Quote(id string) string {
	return fmt.Sprintf(`"%s"`, strings.Replace(id, `"`, `\"`, -1))
}

type CertGraphNodeByName []graph.Node

func (a CertGraphNodeByName) Len() int      { return len(a) }
func (a CertGraphNodeByName) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a CertGraphNodeByName) Less(i, j int) bool {
	lhs := a[i].(CertGraphNode)
	rhs := a[j].(CertGraphNode)

	if lhs.GetCertKeyPair() != nil && rhs.GetCertKeyPair() == nil {
		return true
	}
	if lhs.GetCABundle() != nil && rhs.GetCABundle() == nil {
		return false
	}

	return strings.Compare(lhs.Name(), rhs.Name()) < 0
}
