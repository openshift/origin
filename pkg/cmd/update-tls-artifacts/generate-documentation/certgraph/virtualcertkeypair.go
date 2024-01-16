package certgraph

import (
	"fmt"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

// a virtualCertSource is a cert that we know exists, but we do not know where it's cert/key pair is from.  We may not
// even have the key anymore
type virtualCertSource struct {
	coordinates CertCoordinates
	note        string
	sources     []string
}

func newVirtualCertSource(name string) *virtualCertSource {
	commonName := name[len("cert/"):]
	return &virtualCertSource{
		coordinates: CertCoordinates{
			CertKeyPair: certgraphapi.CertKeyPair{
				Name: commonName,
				Spec: certgraphapi.CertKeyPairSpec{
					CertMetadata: certgraphapi.CertKeyMetadata{
						CertIdentifier: certgraphapi.CertIdentifier{
							CommonName: commonName,
						},
					},
				},
				Status: certgraphapi.CertKeyPairStatus{},
			},
		},
	}
}

func (r *virtualCertSource) Name() string {
	return "certificate-no-key/" + r.coordinates.Name()
}

func (r *virtualCertSource) GetCABundle() *certgraphapi.CertificateAuthorityBundle {
	return nil
}

func (r *virtualCertSource) GetCertKeyPair() *certgraphapi.CertKeyPair {
	return &r.coordinates.CertKeyPair
}

func (s *virtualCertSource) Add(resources Resources) Resource {
	resources.Add(s)
	return s
}

func (s *virtualCertSource) From(source string) Resource {
	s.sources = append(s.sources, source)
	return s
}

func (s *virtualCertSource) Note(note string) Resource {
	s.note = note
	return s
}

func (s *virtualCertSource) String() string {
	return fmt.Sprintf("%v%s", s.coordinates, s.note)
}

func (s *virtualCertSource) GetNote() string {
	return s.note
}

func (s *virtualCertSource) SourceNames() []string {
	if len(s.sources) > 0 {
		return s.sources
	}

	return []string{}
}
