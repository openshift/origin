package validation

import (
	"regexp"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	. "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	. "github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"

	. "github.com/openshift/origin/pkg/template/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
	. "github.com/openshift/origin/pkg/route/api/validation"
)

var parameterNameExp = regexp.MustCompile(`^[a-zA-Z0-9\_]+$`)

// ValidateParameter tests if required fields in the Parameter are set.
func ValidateParameter(param *Parameter) (errs ErrorList) {
	if len(param.Name) == 0 {
		errs = append(errs, NewFieldRequired("name", ""))
		return
	}
	if !parameterNameExp.MatchString(param.Name) {
		errs = append(errs, NewFieldInvalid("name", param.Name))
	}
	return
}

// ValidateTemplate tests if required fields in the Template are set.
func ValidateTemplate(template *Template) (errs ErrorList) {
	if len(template.ID) == 0 {
		errs = append(errs, NewFieldRequired("id", template.ID))
	}
	for i, item := range template.Items {
		err := ErrorList{}
		switch obj := item.Object.(type) {
		case *kubeapi.ReplicationController:
			err = ValidateReplicationController(obj)
		case *kubeapi.Pod:
			err = ValidatePod(obj)
		case *kubeapi.Service:
			err = ValidateService(obj)
                case *routeapi.Route:
                        err = ValidateRoute(obj)
		default:
			// Pass-through unknown types.
		}
		errs = append(errs, err.PrefixIndex(i).Prefix("items")...)
	}
	for i := range template.Parameters {
		paramErr := ValidateParameter(&template.Parameters[i])
		errs = append(errs, paramErr.PrefixIndex(i).Prefix("parameters")...)
	}
	return
}
