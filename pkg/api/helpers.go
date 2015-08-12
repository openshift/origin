package api

import (
	"fmt"
	"strings"

	"k8s.io/kubernetes/pkg/api/validation"
)

var NameMayNotBe = []string{".", ".."}
var NameMayNotContain = []string{"/", "%"}

func MinimalNameRequirements(name string, prefix bool) (bool, string) {
	for _, illegalName := range NameMayNotBe {
		if name == illegalName {
			return false, fmt.Sprintf(`name may not be %q`, illegalName)
		}
	}

	for _, illegalContent := range NameMayNotContain {
		if strings.Contains(name, illegalContent) {
			return false, fmt.Sprintf(`name may not contain %q`, illegalContent)
		}
	}

	return true, ""
}

// GetNameValidationFunc returns a name validation function that includes the standard restrictions we want for all types
func GetNameValidationFunc(nameFunc validation.ValidateNameFunc) validation.ValidateNameFunc {
	return func(name string, prefix bool) (bool, string) {
		if ok, reason := MinimalNameRequirements(name, prefix); !ok {
			return ok, reason
		}

		return nameFunc(name, prefix)
	}
}
