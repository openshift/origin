package autoregenerate_after_expiry

import "github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadatainterfaces"

const annotationName string = "certificates.openshift.io/auto-regenerate-after-offline-expiry"

type AutoRegenerateAfterOfflineExpiryRequirement struct{}

func NewAutoRegenerateAfterOfflineExpiryRequirement() tlsmetadatainterfaces.Requirement {

	md := tlsmetadatainterfaces.NewMarkdown("")
	md.Text("Acknowledging that a cert/key pair or CA bundle can auto-regenerate after it expires offline means")
	md.Text("that if the cluster is shut down until the certificate expires, when the machines are restarted")
	md.Text("the cluster will automatically create new cert/key pairs or update CA bundles as required without human")
	md.Text("intervention.")
	md.Textf("To assert that a particular cert/key pair or CA bundle can do this, add the %q annotation to the secret or configmap and ",
		annotationName)
	md.Text("setting the value of the annotation a github link to the PR adding the annotation.")
	md.Text("This assertion also means that you have")
	md.OrderedListStart()
	md.NewOrderedListItem()
	md.Text("Manually tested that this works or seen someone else manually test that this works.  AND")
	md.NewOrderedListItem()
	md.Text("Written an automated e2e job that your team has an alert for and is a blocking GA criteria, and/or")
	md.Text("QE has required test every release that ensures the functionality works every release.")
	md.OrderedListEnd()
	md.Text("Links should be provided in the PR adding the annotation.")

	return tlsmetadatainterfaces.NewAnnotationRequirement(
		// requirement name
		"autoregenerate-after-expiry",
		// cert or configmap annotation
		annotationName,
		"Auto Regenerate After Offline Expiry",
		string(md.ExactBytes()),
	)
}
