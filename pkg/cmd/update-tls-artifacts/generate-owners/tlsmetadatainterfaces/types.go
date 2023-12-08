package tlsmetadatainterfaces

import (
	"embed"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

type Requirement interface {
	GetName() string

	// InspectRequirement generates and returns the result for a particular set of raw data
	InspectRequirement(rawData []*certgraphapi.PKIList) (RequirementResult, error)
}

type AnnotationRequirement interface {
	Requirement

	// GetAnnotationName returns annotation name to use
	GetAnnotationName() string
}

type RequirementResult interface {
	GetName() string

	// WriteResultToTLSDir writes the content this requirement expects in directory.
	// tlsDir is the parent directory and must be nested as: <tlsDir>/<GetName()>/<content here>.
	// The content MUST include
	// 1. <tlsDir>/<GetName()>/<GetName()>.md
	// 2. <tlsDir>/<GetName()>/<GetName().json
	// 3. <tlsDir>/violations/<GetName()>/<GetName()>-violations.json
	WriteResultToTLSDir(tlsDir string) error

	// DiffExistingContent compares the content of the result with what currently exists in the tlsDir.
	// returns
	//   string representation to display to user (ideally a diff)
	//   bool that is true when content matches and false when content does not match
	//   error which non-nil ONLY when the comparison itself could not complete.  A completed diff that is non-zero is not an error
	DiffExistingContent(tlsDir string) (string, bool, error)

	// HaveViolationsRegressed compares the violations of the result with was passed in and returns
	// allViolationsFS is the tls/violations/<GetName> directory
	// returns
	//   string representation to display to user (ideally a diff of what is worse)
	//   bool that is true when no regressions have been introduced and false when content has gotten worse
	//   error which non-nil ONLY when the comparison itself could not complete.  A completed check that is non-zero is not an error
	HaveViolationsRegressed(allViolationsFS embed.FS) ([]string, bool, error)
}
