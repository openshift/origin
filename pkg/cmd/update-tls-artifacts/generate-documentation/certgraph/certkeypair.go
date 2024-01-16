package certgraph

import (
	"fmt"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

type CertCoordinates struct {
	CertKeyPair certgraphapi.CertKeyPair
}

func (c CertCoordinates) String() string {
	return c.CertKeyPair.Name
}

func (c CertCoordinates) Name() string {
	return c.CertKeyPair.Spec.CertMetadata.CertIdentifier.CommonName
}

type certSource struct {
	coordinates CertCoordinates
	note        string
	sources     []string
}

func (r certSource) Name() string {
	return "cert/" + r.coordinates.Name()
}

func (r *certSource) GetCABundle() *certgraphapi.CertificateAuthorityBundle {
	return nil
}

func (r *certSource) GetCertKeyPair() *certgraphapi.CertKeyPair {
	return &r.coordinates.CertKeyPair
}

func (s *certSource) Add(resources Resources) Resource {
	resources.Add(s)
	return s
}

func (s *certSource) From(source string) Resource {
	s.sources = append(s.sources, source)
	return s
}

func (s *certSource) Note(note string) Resource {
	s.note = note
	return s
}

func (s *certSource) String() string {
	return fmt.Sprintf("%v%s", s.coordinates, s.note)
}

func (s *certSource) GetNote() string {
	return s.note
}

func (s *certSource) SourceNames() []string {
	if len(s.sources) > 0 {
		return s.sources
	}
	if s.coordinates.CertKeyPair.Spec.CertMetadata.CertIdentifier.Issuer == nil {
		return []string{}
	}

	return []string{
		"cert/" + s.coordinates.CertKeyPair.Spec.CertMetadata.CertIdentifier.Issuer.CommonName,
	}
}
