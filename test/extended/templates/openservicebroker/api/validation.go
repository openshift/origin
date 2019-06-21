package api

import (
	"regexp"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

var uuidRegex = regexp.MustCompile("^(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")

func ValidateUUID(path *field.Path, uuid string) field.ErrorList {
	if uuidRegex.MatchString(uuid) {
		return nil
	}
	return field.ErrorList{field.Invalid(path, uuid, "must be a valid UUID")}
}
