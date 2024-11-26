package autoregenerate_after_expiry

import (
	"github.com/openshift/library-go/pkg/markdown"
	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadata/testcase"
	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadatainterfaces"
)

const annotationName string = "certificates.openshift.io/auto-regenerate-after-offline-expiry"

type AutoRegenerateAfterOfflineExpiryRequirement struct{}

func NewAutoRegenerateAfterOfflineExpiryRequirement() tlsmetadatainterfaces.Requirement {

	md := markdown.NewMarkdown("")
	md.Text("Acknowledging that a cert/key pair or CA bundle can auto-regenerate after it expires offline means")
	md.Text("that if the cluster is shut down until the certificate expires, when the machines are restarted")
	md.Text("the cluster will automatically create new cert/key pairs or update CA bundles as required without human")
	md.Text("intervention.")
	md.Text("")
	md.Text("To assert that a particular cert/key pair or CA bundle can do this, add the annotation to the secret or configmap.")
	md.Text("```yaml")
	md.Text("  annotations:")
	md.Textf("    %v: https//github.com/link/to/pr/adding/annotation", annotationName)
	md.Text("```")
	md.Text("")
	md.Text("This assertion means that you have")
	md.OrderedListStart()
	md.NewOrderedListItem()
	md.Text("Manually tested that this works or seen someone else manually test that this works.  AND")
	md.NewOrderedListItem()
	md.Text("Written an automated e2e test to ensure this PKI artifact is function that is a blocking GA criteria, and/or")
	md.NewOrderedListItem()
	md.Text("QE has required test every release that ensures the functionality works every release.")
	md.Textf("This TLS artifact has associated test name annotation (%q).", testcase.AnnotationName)
	md.OrderedListEnd()
	md.Text("If you have not done this, you should not merge the annotation.")

	return tlsmetadatainterfaces.NewAnnotationRequirement(
		// requirement name
		"autoregenerate-after-expiry",
		// cert or configmap annotation
		annotationName,
		"Auto Regenerate After Offline Expiry",
		string(md.ExactBytes()),
	)
}
