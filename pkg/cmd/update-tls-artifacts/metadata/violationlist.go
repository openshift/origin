package metadata

import (
	"fmt"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

type ViolationList []Violation

func (l ViolationList) DiffWithExistingJSON(parentDir string) error {
	var errCombined error
	for _, v := range l {
		if err := v.DiffWithExistingJSON(parentDir); err != nil {
			errCombined = fmt.Errorf("%v\n %s: %v", errCombined, v.Name, err)
		}
	}
	return errCombined
}

func (l ViolationList) DiffWithExistingMarkdown(parentDir string) error {
	var errCombined error
	for _, v := range l {
		if err := v.DiffWithExistingMarkdown(parentDir); err != nil {
			errCombined = fmt.Errorf("%v\n %s: %v", errCombined, v.Name, err)
		}
	}
	return errCombined
}

func (l ViolationList) WriteJSONFiles(parentDir string) error {
	for _, v := range l {
		if err := v.WriteJSONFile(parentDir); err != nil {
			return err
		}
	}
	return nil
}

func (l ViolationList) WriteMarkdownFiles(dir string) error {
	for _, v := range l {
		if err := v.WriteMarkdownFile(dir); err != nil {
			return err
		}
	}
	return nil
}

func GenerateViolationList(pkiInfo *certgraphapi.PKIRegistryInfo, reqs ...Requirement) (ViolationList, error) {
	result := ViolationList{}

	for _, req := range reqs {
		violation, err := req.NewViolation(req.Name, pkiInfo)
		if err != nil {
			return result, fmt.Errorf("%s: %v", violation.Name, err)
		}
		result = append(result, violation)
	}
	return result, nil
}
