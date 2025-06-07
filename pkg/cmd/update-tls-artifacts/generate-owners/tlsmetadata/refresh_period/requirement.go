package refresh_period

import (
	"github.com/openshift/library-go/pkg/markdown"
	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadata/testcase"
	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadatainterfaces"
)

const annotationName string = "certificates.openshift.io/refresh-period"

type RefreshPeriodRequirement struct{}

func NewRefreshPeriodRequirement() tlsmetadatainterfaces.Requirement {

	md := markdown.NewMarkdown("")
	md.Text("Acknowledging that a cert/key pair or CA bundle can be refreshed means")
	md.Text("that certificate is being updated before its expiration date as required without human")
	md.Text("intervention.")
	md.Text("")
	md.Text("To assert that a particular cert/key pair or CA bundle can be refreshed, add the annotation to the secret or configmap.")
	md.Text("```yaml")
	md.Text("  annotations:")
	md.Textf("    %v: <refresh period, e.g. 15d or 2y>", annotationName)
	md.Text("```")
	md.Text("")
	md.Text("This assertion means that you have")
	md.OrderedListStart()
	md.NewOrderedListItem()
	md.Text("Manually tested that this works or seen someone else manually test that this works.  AND")
	md.NewOrderedListItem()
	md.Text("Written an automated e2e test to ensure this PKI artifact is function that is a blocking GA criteria, and/or")
	md.Text("QE has required test every release that ensures the functionality works every release.")
	md.NewOrderedListItem()
	md.Textf("This TLS artifact has associated test name annotation (%q).", testcase.AnnotationName)
	md.OrderedListEnd()
	md.Text("If you have not done this, you should not merge the annotation.")

	return tlsmetadatainterfaces.NewAnnotationRequirement(
		// requirement name
		"refresh-period",
		// cert or configmap annotation
		annotationName,
		"Refresh Period",
		string(md.ExactBytes()),
	)
}
