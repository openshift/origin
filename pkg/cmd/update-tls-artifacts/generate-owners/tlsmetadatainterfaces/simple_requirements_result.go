package tlsmetadatainterfaces

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/go-cmp/cmp"
)

type SimpleRequirementsResult struct {
	name string

	statusJSON     []byte
	statusMarkdown []byte
	violationJSON  []byte
}

func NewRequirementResult(name string, statusJSON, statusMarkdown, violationJSON []byte) (*SimpleRequirementsResult, error) {
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
