package testcase

import (
	"github.com/openshift/library-go/pkg/markdown"
	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadatainterfaces"
)

const AnnotationName string = "certificates.openshift.io/test-name"

type TestNameRequirement struct{}

func NewTestNameRequirement() tlsmetadatainterfaces.Requirement {

	md := markdown.NewMarkdown("")
	md.Text("Every TLS artifact should be associated with a test, which checks that cert key pair.")
	md.Text("or CA bundle is being properly issued, refreshed, regenerated while offline")
	md.Text("and correctly reloaded.")
	md.Text("")
	md.Text("To assert that a particular cert/key pair or CA bundle is being tested, add the annotation to the secret or configmap.")
	md.Text("```yaml")
	md.Text("  annotations:")
	md.Textf("    %v: name of e2e test that ensures the PKI artifact functions properly", AnnotationName)
	md.Text("```")
	md.Text("")
	md.Text("This assertion means that you have")
	md.OrderedListStart()
	md.NewOrderedListItem()
	md.Text("Manually tested that this works or seen someone else manually test that this works.  AND")
	md.NewOrderedListItem()
	md.Text("Written an automated e2e test to ensure this PKI artifact is function that is a blocking GA criteria, and/or")
	md.Text("QE has required test every release that ensures the functionality works every release.")
	md.OrderedListEnd()
	md.Text("If you have not done this, you should not merge the annotation.")

	return tlsmetadatainterfaces.NewAnnotationRequirement(
		// requirement name
		"testcase",
		// cert or configmap annotation
		AnnotationName,
		"Test Cases",
		string(md.ExactBytes()),
	)
}
