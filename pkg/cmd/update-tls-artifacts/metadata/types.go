package metadata

import (
	"bytes"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

const unknownOwner = "Unknown"

type Requirement interface {
	GetName() string
	GetViolation(name string, pkiInfo *certgraphapi.PKIRegistryInfo) (Violation, error)
	GenerateMarkdown(pkiInfo *certgraphapi.PKIRegistryInfo) ([]byte, error)
	DiffCertKeyPair(actual, expected certgraphapi.PKIRegistryCertKeyPairInfo) string
	DiffCABundle(actual, expected certgraphapi.PKIRegistryCertificateAuthorityInfo) string
}

type Violation struct {
	Name     string
	Markdown []byte
	Registry *certgraphapi.PKIRegistryInfo
}

type Markdown struct {
	title           string
	tableOfContents *bytes.Buffer
	body            *bytes.Buffer

	orderedListDepth      int
	orderedListItemStart  bool
	orderedListItemNumber []int
}
