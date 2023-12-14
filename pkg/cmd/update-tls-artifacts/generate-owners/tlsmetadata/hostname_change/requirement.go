package hostname_change

import "github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadatainterfaces"

const annotationName string = "certificates.openshift.io/supports-offline-hostname-change"

type SupportsOfflineHostnameChange struct{}

func NewSupportsOfflineHostnameChange() tlsmetadatainterfaces.Requirement {

	md := tlsmetadatainterfaces.NewMarkdown("")
	md.Text("Offline hostname change is an SNO feature driven using tool (provide link here) while a cluster is not running.")
	md.Text("")
	md.Textf("Adding the %q annotation means that the cert/key pair or CA bundle has been verified to function properly", annotationName)
	md.Text("after the regeneration with a new hostname happens.")
	md.Text("The value of the annotation must be a link to the PR adding to the annotation.")
	md.Text("This needs to cover not just serving certificate DNS and IPs, but some client certificates include hostname")
	md.Text("embedded in names, and CA bundles need to be able to verify the new client and serving certificates as well.")
	md.Text("")
	md.Textf("Setting `.annotation[%q]=<URL to PR>` means that you have:", annotationName)
	md.OrderedListStart()
	md.NewOrderedListItem()
	md.Text("Manually tested that this particular TLS artifact worked properly after a hostname is updated using this tool.")
	md.Text("If the manual approach is taken, then QE must include this manual test in their \"must pass prior to ship\" bucket.  OR")
	md.NewOrderedListItem()
	md.Text("Written an automated e2e job that runs the tool AND has a test that explicitly checks the TLS artifact in question.")
	md.Text("The generic bucket MAY test this, but before adding this annotation you MUST be able to indicate which precise")
	md.Text("test ensures that this TLS artifact is functioning properly.")
	md.OrderedListEnd()

	return tlsmetadatainterfaces.NewAnnotationRequirement(
		// requirement name
		"offline-hostname-change",
		// cert or configmap annotation
		annotationName,
		"Supports Offline Hostname Change",
		string(md.ExactBytes()),
	)
}
