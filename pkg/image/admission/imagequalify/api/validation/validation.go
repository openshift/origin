package validation

import (
	"fmt"
	"regexp"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/image/admission/imagequalify/api"
)

const (
	patternCharSet = `^(?:[a-zA-Z0-9_@:/\.\-\*]+)$`
)

var patternRegexp = regexp.MustCompile(patternCharSet)
var patternMatchError = fmt.Sprintf("pattern must match %q", patternCharSet)

func Validate(config *api.ImageQualifyConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	if config == nil {
		return allErrs
	}
	for i, rule := range config.Rules {
		if rule.Pattern == "" {
			allErrs = append(allErrs, field.Required(field.NewPath(api.PluginName, "rules").Index(i).Child("pattern"), ""))
		}
		if rule.Pattern != "" {
			if !patternRegexp.MatchString(rule.Pattern) {
				allErrs = append(allErrs, field.Invalid(field.NewPath(api.PluginName, "rules").Index(i).Child("pattern"), rule.Pattern, patternMatchError))
			}
		}
		if rule.Domain == "" {
			allErrs = append(allErrs, field.Required(field.NewPath(api.PluginName, "rules").Index(i).Child("domain"), ""))
		}
		if rule.Domain != "" {
			if err := validateDomain(rule.Domain); err != nil {
				allErrs = append(allErrs, field.Invalid(field.NewPath(api.PluginName, "rules").Index(i).Child("domain"), rule.Domain, err.Error()))
			}
		}
	}
	return allErrs
}
