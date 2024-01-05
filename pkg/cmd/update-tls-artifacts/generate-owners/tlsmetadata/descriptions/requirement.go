package descriptions

import (
	"github.com/openshift/api/annotations"
	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadatainterfaces"
)

type DescriptionRequirements struct{}

func NewDescriptionRequirement() tlsmetadatainterfaces.Requirement {

	md := tlsmetadatainterfaces.NewMarkdown("")
	md.Text("TLS artifacts must have user-facing descriptions on their in-cluster resources.")
	md.Text("These descriptions must be in the style of API documentation and must include")
	md.OrderedListStart()
	md.NewOrderedListItem()
	md.Text("Which connections a CA bundle can be used to verify.")
	md.NewOrderedListItem()
	md.Text("What kind of certificates a signer will sign for.")
	md.NewOrderedListItem()
	md.Text("Which names and IPs a serving certificate terminates.")
	md.NewOrderedListItem()
	md.Text("Which subject (user and group) a client certificate is created for.")
	md.NewOrderedListItem()
	md.Text("Which binary and flags is this certificate wired to.")
	md.OrderedListEnd()
	md.Text("")
	md.Textf("To create a description, set the `%v` annotation to the markdown formatted string describing your TLS artifact. ",
		annotations.OpenShiftDescription)

	return tlsmetadatainterfaces.NewAnnotationRequirement(
		"descriptions",
		annotations.OpenShiftDescription,
		"Description of TLS Artifacts",
		string(md.ExactBytes()),
	)
}
