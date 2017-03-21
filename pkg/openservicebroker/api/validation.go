package api

import (
	"regexp"

	"k8s.io/kubernetes/pkg/util/validation/field"
)

func ValidateProvisionRequest(preq *ProvisionRequest) field.ErrorList {
	errors := ValidateUUID(field.NewPath("service_id"), preq.ServiceID)
	errors = append(errors, ValidateUUID(field.NewPath("plan_id"), preq.PlanID)...)

	return errors
}

func ValidateBindRequest(breq *BindRequest) field.ErrorList {
	errors := ValidateUUID(field.NewPath("service_id"), breq.ServiceID)
	errors = append(errors, ValidateUUID(field.NewPath("plan_id"), breq.PlanID)...)

	return errors
}

var uuidRegex = regexp.MustCompile("^(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")

func ValidateUUID(path *field.Path, uuid string) field.ErrorList {
	if uuidRegex.MatchString(uuid) {
		return nil
	}
	return field.ErrorList{field.Invalid(path, uuid, "must be a valid UUID")}
}
