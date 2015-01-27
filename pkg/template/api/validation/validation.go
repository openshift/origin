package validation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/openshift/origin/pkg/template/api"
	"github.com/openshift/origin/pkg/util"
)

var parameterNameExp = regexp.MustCompile(`^[a-zA-Z0-9\_]+$`)

// ValidateParameter tests if required fields in the Parameter are set.
func ValidateParameter(param *api.Parameter) (errs errors.ValidationErrorList) {
	if len(param.Name) == 0 {
		errs = append(errs, errors.NewFieldRequired("name", ""))
		return
	}
	if !parameterNameExp.MatchString(param.Name) {
		errs = append(errs, errors.NewFieldInvalid("name", param.Name, fmt.Sprintf("does not match %v", parameterNameExp)))
	}
	return
}

// ValidateTemplate tests if required fields in the Template are set.
func ValidateTemplate(template *api.Template) (errs errors.ValidationErrorList) {
	if len(template.Name) == 0 {
		errs = append(errs, errors.NewFieldRequired("name", template.Name))
	}
	for i := range template.Parameters {
		paramErr := ValidateParameter(&template.Parameters[i])
		errs = append(errs, paramErr.PrefixIndex(i).Prefix("parameters")...)
	}
	for _, obj := range template.Items {
		errs = append(errs, util.ValidateObject(obj)...)
	}
	return
}

func filter(errs errors.ValidationErrorList, prefix string) errors.ValidationErrorList {
	if errs == nil {
		return errs
	}
	next := errors.ValidationErrorList{}
	for _, err := range errs {
		ve, ok := err.(*errors.ValidationError)
		if ok && strings.HasPrefix(ve.Field, prefix) {
			continue
		}
		next = append(next, err)
	}
	return next
}
