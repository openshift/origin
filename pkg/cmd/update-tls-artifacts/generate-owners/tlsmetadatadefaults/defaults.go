package tlsmetadatadefaults

import (
	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadata/autoregenerate_after_expiry"
	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadata/descriptions"
	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadata/ownership"
	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadatainterfaces"
)

func GetDefaultTLSRequirements() []tlsmetadatainterfaces.Requirement {
	return []tlsmetadatainterfaces.Requirement{
		ownership.NewOwnerRequirement(),
		autoregenerate_after_expiry.NewAutoRegenerateAfterOfflineExpiryRequirement(),
		descriptions.NewDescriptionRequirement(),
	}
}
