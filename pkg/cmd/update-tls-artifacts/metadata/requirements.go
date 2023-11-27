package metadata

import (
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

type secretCompareFunc func(actual, expected certgraphapi.PKIRegistryCertKeyPairInfo) error
type configMapCompareFunc func(actual, expected certgraphapi.PKIRegistryCertificateAuthorityInfo) error

type Requirement struct {
	Name                 string
	SecretCompareFunc    secretCompareFunc
	ConfigMapCompareFunc configMapCompareFunc
	NewViolation         ViolationFunc
}

var (
	Required = []Requirement{OwnerRequriement}
	Optional = []Requirement{DescriptionRequriement}

	OwnerRequriement = Requirement{
		Name:                 "ownership",
		NewViolation:         newOwnerViolation,
		SecretCompareFunc:    diffCertKeyPairOwners,
		ConfigMapCompareFunc: diffCABundleOwners,
	}
	DescriptionRequriement = Requirement{
		Name:                 "description",
		NewViolation:         newDescriptionViolation,
		SecretCompareFunc:    diffCertKeyPairDescription,
		ConfigMapCompareFunc: diffCABundleDescription,
	}
)
