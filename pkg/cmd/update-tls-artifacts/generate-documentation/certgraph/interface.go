package certgraph

import (
	"fmt"

	"github.com/gonum/graph"
	"github.com/gonum/graph/encoding/dot"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

func NewResources() Resources {
	return &resourcesImpl{}
}

func NewCert(certKeyPair certgraphapi.CertKeyPair) Resource {
	return &certSource{coordinates: NewCertCoordinates(certKeyPair)}
}

func NewCABundle(caBundle certgraphapi.CertificateAuthorityBundle) Resource {
	return &caBundleSource{coordinates: NewCABundleCoordinates(caBundle)}
}

type Resource interface {
	Name() string
	Add(resources Resources) Resource
	From(string) Resource
	Note(note string) Resource

	fmt.Stringer
	GetNote() string
	GetCABundle() *certgraphapi.CertificateAuthorityBundle
	GetCertKeyPair() *certgraphapi.CertKeyPair
	SourceNames() []string
}

type Resources interface {
	Add(resource Resource)
	AllResources() []Resource
	Roots() []Resource
	NewGraph() (graph.Directed, error)
}

type CertGraphNode interface {
	Name() string
	GetCABundle() *certgraphapi.CertificateAuthorityBundle
	GetCertKeyPair() *certgraphapi.CertKeyPair
}

func NewCertCoordinates(certKeyPair certgraphapi.CertKeyPair) CertCoordinates {
	return CertCoordinates{
		CertKeyPair: certKeyPair,
	}
}

func NewCABundleCoordinates(caBundle certgraphapi.CertificateAuthorityBundle) CABundleCoordinates {
	return CABundleCoordinates{
		CABundle: caBundle,
	}
}

func GraphForPKIList(pkiList *certgraphapi.PKIList) (graph.Directed, error) {
	allNodes := NewResources()
	for _, cert := range pkiList.CertKeyPairs.Items {
		allNodes.Add(NewCert(cert))
	}
	for _, caBundle := range pkiList.CertificateAuthorityBundles.Items {
		allNodes.Add(NewCABundle(caBundle))
	}
	return allNodes.NewGraph()
}

func DOTForPKIList(pkiList *certgraphapi.PKIList) (string, error) {
	graph, err := GraphForPKIList(pkiList)
	if err != nil {
		return "", err
	}

	data, err := dot.Marshal(graph, Quote("OpenShift Certificates"), "", "  ", false)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
