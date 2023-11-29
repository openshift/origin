package tlsmetadata

import (
	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadata/description"
	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadata/ownership"
	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadatainterfaces"
)

var (
	Required = []string{"owner"}
	All      = []tlsmetadatainterfaces.Requirement{ownership.NewOwnerRequirement(), description.NewDescriptionRequirement()}
)
