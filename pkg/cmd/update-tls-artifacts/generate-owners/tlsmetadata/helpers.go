package tlsmetadata

import (
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphutils"
	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadatainterfaces"
)

const UnknownOwner = "Unknown"

type requirementsResult struct {
	delegate *tlsmetadatainterfaces.SimpleRequirementsResult

	violations *certgraphapi.PKIRegistryInfo
}

func AnnotationValue(whitelistedAnnotations []certgraphapi.AnnotationValue, key string) (string, bool) {
	for _, curr := range whitelistedAnnotations {
		if curr.Key == key {
			return curr.Value, true
		}
	}

	return "", false
}

func NewRequirementResult(name string, statusJSON, statusMarkdown, violationJSON []byte) (tlsmetadatainterfaces.RequirementResult, error) {
	delegate, err := tlsmetadatainterfaces.NewRequirementResult(name, statusJSON, statusMarkdown, violationJSON)
	if err != nil {
		return nil, err
	}

	// technically could be passed in, but I think this might make the division easier.  we'll see
	resultingViolations := &certgraphapi.PKIRegistryInfo{}
	if err := json.Unmarshal(violationJSON, resultingViolations); err != nil {
		return nil, fmt.Errorf("error decoding violation content for %v: %w", name, err)
	}

	return &requirementsResult{
		delegate:   delegate,
		violations: resultingViolations,
	}, nil
}

// TODO this might be generic for "does it have this annotation values"
func (r requirementsResult) HaveViolationsRegressed(allViolationsFS embed.FS) ([]string, bool, error) {
	existingViolationJSONBytes, err := allViolationsFS.ReadFile(filepath.Join("violations", r.GetName(), fmt.Sprintf("%s-violations.json", r.GetName())))
	if err != nil {
		return nil, false, fmt.Errorf("error reading existing content for %v: %w", r.GetName(), err)
	}
	existingViolations := &certgraphapi.PKIRegistryInfo{}
	if err := json.Unmarshal(existingViolationJSONBytes, existingViolations); err != nil {
		return nil, false, fmt.Errorf("error decoding existing content for %v: %w", r.GetName(), err)
	}

	regressions := []string{}
	for _, currCertKeyPair := range r.violations.CertKeyPairs {
		currLocation := currCertKeyPair.SecretLocation
		_, err := certgraphutils.LocateCertKeyPair(currLocation, existingViolations.CertKeyPairs)
		if err != nil {
			// this means it wasn't found
			regressions = append(regressions,
				fmt.Sprintf("requirment/%v: --namespace=%v secret/%v regressed and does not have an owner", r.GetName(), currLocation.Namespace, currLocation.Name),
			)
		}
	}

	for _, currCABundle := range r.violations.CertificateAuthorityBundles {
		currLocation := currCABundle.ConfigMapLocation
		_, err := certgraphutils.LocateCertificateAuthorityBundle(currLocation, existingViolations.CertificateAuthorityBundles)
		if err != nil {
			// this means it wasn't found
			regressions = append(regressions,
				fmt.Sprintf("requirment/%v: --namespace=%v configmap/%v regressed and does not have an owner", r.GetName(), currLocation.Namespace, currLocation.Name),
			)
		}
	}

	if len(regressions) > 0 {
		return regressions, true, nil
	}
	return nil, false, nil
}

func (r requirementsResult) GetName() string {
	return r.delegate.GetName()
}

func (r requirementsResult) WriteResultToTLSDir(tlsDir string) error {
	return r.delegate.WriteResultToTLSDir(tlsDir)
}

func (r requirementsResult) DiffExistingContent(tlsDir string) (string, bool, error) {
	return r.delegate.DiffExistingContent(tlsDir)
}
