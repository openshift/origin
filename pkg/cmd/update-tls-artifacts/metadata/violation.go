package metadata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/go-cmp/cmp"
)

func (v Violation) getJSONFilePath(parentDir string) string {
	return filepath.Join(parentDir, fmt.Sprintf("%s-violations.json", v.Name))
}

func (v Violation) getMarkdownFilePath(parentDir string) string {
	return filepath.Join(parentDir, fmt.Sprintf("%s.md", v.Name))
}

func (v Violation) DiffWithExistingJSON(parentDir string) error {
	violationJSONBytes, err := json.MarshalIndent(v.Registry, "", "    ")
	if err != nil {
		return err
	}

	existingViolationsJSONBytes, err := os.ReadFile(v.getJSONFilePath(parentDir))
	switch {
	case os.IsNotExist(err): // do nothing
	case err != nil:
		return err
	}
	if diff := cmp.Diff(existingViolationsJSONBytes, violationJSONBytes); len(diff) > 0 {
		return fmt.Errorf(diff)
	}
	return nil
}

func (v Violation) DiffWithExistingMarkdown(parentDir string) error {
	existingViolationsMarkdownBytes, err := os.ReadFile(v.getMarkdownFilePath(parentDir))
	switch {
	case os.IsNotExist(err): // do nothing
	case err != nil:
		return err
	}
	if diff := cmp.Diff(existingViolationsMarkdownBytes, v.Markdown); len(diff) > 0 {
		return fmt.Errorf(diff)
	}
	return nil
}

func (v Violation) WriteJSONFile(parentDir string) error {
	violationJSONBytes, err := json.MarshalIndent(v.Registry, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(v.getJSONFilePath(parentDir), violationJSONBytes, 0644)
}

func (v Violation) WriteMarkdownFile(parentDir string) error {
	return os.WriteFile(v.getMarkdownFilePath(parentDir), v.Markdown, 0644)
}
