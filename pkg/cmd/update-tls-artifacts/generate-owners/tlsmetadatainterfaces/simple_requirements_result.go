package tlsmetadatainterfaces

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphutils"
	"github.com/openshift/origin/pkg/certs"

	"github.com/google/go-cmp/cmp"
)

type SimpleRequirementsResult struct {
	name string

	statusJSON     []byte
	statusMarkdown []byte
	violationJSON  []byte
}

func NewRequirementResult(name string, statusJSON, statusMarkdown, violationJSON []byte) (RequirementResult, error) {
	if len(name) == 0 {
		return nil, fmt.Errorf("missing name for result")
	}
	if len(statusJSON) == 0 {
		return nil, fmt.Errorf("result for %v missing statusJSON", name)
	}
	if len(statusMarkdown) == 0 {
		return nil, fmt.Errorf("result for %v missing statusJSON", name)
	}
	if len(violationJSON) == 0 {
		return nil, fmt.Errorf("result for %v missing statusJSON", name)
	}

	return &SimpleRequirementsResult{
		name:           name,
		statusJSON:     statusJSON,
		statusMarkdown: statusMarkdown,
		violationJSON:  violationJSON,
	}, nil
}

func (s SimpleRequirementsResult) WriteResultToTLSDir(tlsDir string) error {
	if err := os.WriteFile(s.jsonFilename(tlsDir), s.statusJSON, 0644); err != nil {
		return err
	}

	if err := os.WriteFile(s.markdownFilename(tlsDir), s.statusMarkdown, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(s.violationsFilename(tlsDir), s.violationJSON, 0644); err != nil {
		return err
	}

	return nil
}

func (s SimpleRequirementsResult) DiffExistingContent(tlsDir string) (string, bool, error) {
	existingStatusJSONBytes, err := os.ReadFile(s.jsonFilename(tlsDir))
	switch {
	case os.IsNotExist(err): // do nothing
	case err != nil:
		return "", false, err
	}
	if diff := cmp.Diff(existingStatusJSONBytes, s.statusJSON); len(diff) > 0 {
		return diff, false, nil
	}

	existingViolationsJSONBytes, err := os.ReadFile(s.violationsFilename(tlsDir))
	switch {
	case os.IsNotExist(err): // do nothing
	case err != nil:
		return "", false, err
	}
	if diff := cmp.Diff(existingViolationsJSONBytes, s.violationJSON); len(diff) > 0 {
		return diff, false, nil
	}

	existingStatusMarkdown, err := os.ReadFile(s.markdownFilename(tlsDir))
	switch {
	case os.IsNotExist(err): // do nothing
	case err != nil:
		return "", false, err
	}
	if diff := cmp.Diff(existingStatusMarkdown, s.statusMarkdown); len(diff) > 0 {
		return diff, false, nil
	}

	return "", true, nil
}

func (s SimpleRequirementsResult) HaveViolationsRegressed(allViolationsFS embed.FS) ([]string, bool, error) {
	resultingViolations := &certs.PKIRegistryInfo{}
	if err := json.Unmarshal(s.violationJSON, resultingViolations); err != nil {
		return nil, false, fmt.Errorf("error decoding violation content for %v: %w", s.GetName(), err)
	}

	existingViolationJSONBytes, err := allViolationsFS.ReadFile(s.violationsFilename(""))
	if err != nil {
		return nil, false, fmt.Errorf("error reading existing content for %v: %w", s.GetName(), err)
	}
	existingViolations := &certs.PKIRegistryInfo{}
	if err := json.Unmarshal(existingViolationJSONBytes, existingViolations); err != nil {
		return nil, false, fmt.Errorf("error decoding existing content for %v: %w", s.GetName(), err)
	}

	regressions := []string{}
	for _, currCertKeyPair := range resultingViolations.CertKeyPairs {
		if currCertKeyPair.InClusterLocation != nil {
			currLocation := currCertKeyPair.InClusterLocation.SecretLocation
			_, err := certgraphutils.LocateCertKeyPairBySecretLocation(currLocation, existingViolations.CertKeyPairs)
			if err != nil {
				// this means it wasn't found
				regressions = append(regressions,
					fmt.Sprintf("requirment/%v: --namespace=%v secret/%v regressed and does not have an owner", s.GetName(), currLocation.Namespace, currLocation.Name),
				)
			}
		}
		// TODO[vrutkovs]: add currCertKeyPair.OnDiskLocation
	}

	for _, currCABundle := range resultingViolations.CertificateAuthorityBundles {
		if currCABundle.InClusterLocation != nil {
			currLocation := currCABundle.InClusterLocation.ConfigMapLocation
			_, err := certgraphutils.LocateCABundleByConfigMapLocation(currLocation, existingViolations.CertificateAuthorityBundles)
			if err != nil {
				// this means it wasn't found
				regressions = append(regressions,
					fmt.Sprintf("requirment/%v: --namespace=%v configmap/%v regressed and does not have an owner", s.GetName(), currLocation.Namespace, currLocation.Name),
				)
			}
		}
		// TODO[vrutkovs]: add currCABundle.OnDiskLocation
	}

	if len(regressions) > 0 {
		return regressions, true, nil
	}
	return nil, false, nil
}

func (s SimpleRequirementsResult) jsonFilename(tlsDir string) string {
	return filepath.Join(tlsDir, s.GetName(), fmt.Sprintf("%s.json", s.GetName()))
}

func (s SimpleRequirementsResult) markdownFilename(tlsDir string) string {
	return filepath.Join(tlsDir, s.GetName(), fmt.Sprintf("%s.md", s.GetName()))
}

func (s SimpleRequirementsResult) violationsFilename(tlsDir string) string {
	return filepath.Join(tlsDir, "violations", s.GetName(), fmt.Sprintf("%s-violations.json", s.GetName()))
}

func (s SimpleRequirementsResult) GetName() string {
	return s.name
}
