package validation

import (
	"regexp"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/openshift/origin/pkg/template/api"
)

var parameterNameExp = regexp.MustCompile(`^[a-zA-Z0-9\_]+$`)

// ValidateParameter tests if required fields in the Parameter are set.
func ValidateParameter(param *api.Parameter) (errs errors.ValidationErrorList) {
	if len(param.Name) == 0 {
		errs = append(errs, errors.NewFieldRequired("name", ""))
		return
	}
	if !parameterNameExp.MatchString(param.Name) {
		errs = append(errs, errors.NewFieldInvalid("name", param.Name))
	}
	return
}

// ValidateTemplate tests if required fields in the Template are set.
func ValidateTemplate(template *api.Template) (errs errors.ValidationErrorList) {
	if len(template.Name) == 0 {
		errs = append(errs, errors.NewFieldRequired("name", template.ObjectMeta.Name))
	}
	// TODO: Validation of items are now broken as we need to use Typer and Mapper
	//			 parse the proper version/kind and then validate.
	/*
		for i, item := range template.Items {
			err := errors.ValidationErrorList{}
			switch obj := item.Object.(type) {
			case *kapi.ReplicationController:
				err = validation.ValidateReplicationController(obj)
			case *kapi.Pod:
				err = validation.ValidatePod(obj)
			// TODO: ValidateService() now requires registry and context, we should
			// provide them here
			//case *kapi.Service:
			//	err = validation.ValidateService(obj)
			case *routeapi.Route:
				err = routevalidation.ValidateRoute(obj)
			default:
				// Pass-through unknown types.
			}
			// ignore namespace validation errors in templates
			err = filter(err, "namespace")
			errs = append(errs, err.PrefixIndex(i).Prefix("items")...)
		}
	*/
	for i := range template.Parameters {
		paramErr := ValidateParameter(&template.Parameters[i])
		errs = append(errs, paramErr.PrefixIndex(i).Prefix("parameters")...)
	}
	return
}

func filter(errs errors.ValidationErrorList, prefix string) errors.ValidationErrorList {
	if errs == nil {
		return errs
	}
	next := errors.ValidationErrorList{}
	for _, err := range errs {
		ve, ok := err.(errors.ValidationError)
		if ok && strings.HasPrefix(ve.Field, prefix) {
			continue
		}
		next = append(next, err)
	}
	return next
}
