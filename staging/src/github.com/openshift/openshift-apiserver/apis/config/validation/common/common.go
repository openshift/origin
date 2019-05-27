package common

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strconv"
	"unicode"
	"unicode/utf8"

	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/cmd/server/apis/config"
)

func ValidateStringSource(s config.StringSource, fieldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}
	methods := 0
	if len(s.Value) > 0 {
		methods++
	}
	if len(s.File) > 0 {
		methods++
		fileErrors := ValidateFile(s.File, fieldPath.Child("file"))
		validationResults.AddErrors(fileErrors...)

		// If the file was otherwise ok, and its value will be used verbatim, warn about trailing whitespace
		if len(fileErrors) == 0 && len(s.KeyFile) == 0 {
			if data, err := ioutil.ReadFile(s.File); err != nil {
				validationResults.AddErrors(field.Invalid(fieldPath.Child("file"), s.File, fmt.Sprintf("could not read file: %v", err)))
			} else if len(data) > 0 {
				r, _ := utf8.DecodeLastRune(data)
				if unicode.IsSpace(r) {
					validationResults.AddWarnings(field.Invalid(fieldPath.Child("file"), s.File, "contains trailing whitespace which will be included in the value"))
				}
			}
		}
	}
	if len(s.Env) > 0 {
		methods++
	}
	if methods > 1 {
		validationResults.AddErrors(field.Invalid(fieldPath, "", "only one of value, file, and env can be specified"))
	}

	if len(s.KeyFile) > 0 {
		validationResults.AddErrors(ValidateFile(s.KeyFile, fieldPath.Child("keyFile"))...)
	}

	return validationResults
}

func ValidateSpecifiedIP(ipString string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	ip := net.ParseIP(ipString)
	if ip == nil {
		allErrs = append(allErrs, field.Invalid(fldPath, ipString, "must be a valid IP"))
	} else if ip.IsUnspecified() {
		allErrs = append(allErrs, field.Invalid(fldPath, ipString, "cannot be an unspecified IP"))
	}

	return allErrs
}

func ValidateSpecifiedIPPort(ipPortString string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	ipString, portString, err := net.SplitHostPort(ipPortString)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, ipPortString, "must be a valid IP:PORT"))
		return allErrs
	}

	ip := net.ParseIP(ipString)
	if ip == nil {
		allErrs = append(allErrs, field.Invalid(fldPath, ipString, "must be a valid IP"))
	} else if ip.IsUnspecified() {
		allErrs = append(allErrs, field.Invalid(fldPath, ipString, "cannot be an unspecified IP"))
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, portString, "must be a valid port"))
	} else {
		for _, msg := range utilvalidation.IsValidPortNum(port) {
			allErrs = append(allErrs, field.Invalid(fldPath, port, msg))
		}
	}

	return allErrs
}

func ValidateSecureURL(urlString string, fldPath *field.Path) (*url.URL, field.ErrorList) {
	url, urlErrs := ValidateURL(urlString, fldPath)
	if len(urlErrs) == 0 && url.Scheme != "https" {
		urlErrs = append(urlErrs, field.Invalid(fldPath, urlString, "must use https scheme"))
	}
	return url, urlErrs
}

func ValidateURL(urlString string, fldPath *field.Path) (*url.URL, field.ErrorList) {
	allErrs := field.ErrorList{}

	urlObj, err := url.Parse(urlString)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, urlString, "must be a valid URL"))
		return nil, allErrs
	}
	if len(urlObj.Scheme) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, urlString, "must contain a scheme (e.g. https://)"))
	}
	if len(urlObj.Host) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, urlString, "must contain a host"))
	}
	return urlObj, allErrs
}

func ValidateFile(path string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(path) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, ""))
	} else if _, err := os.Stat(path); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, path, fmt.Sprintf("could not read file: %v", err)))
	}

	return allErrs
}

func ValidateDir(path string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(path) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, ""))
	} else {
		fileInfo, err := os.Stat(path)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, path, fmt.Sprintf("could not read info: %v", err)))
		} else if !fileInfo.IsDir() {
			allErrs = append(allErrs, field.Invalid(fldPath, path, "not a directory"))
		}
	}

	return allErrs
}

// TODO: this should just be two return arrays, no need to be clever
type ValidationResults struct {
	Warnings field.ErrorList
	Errors   field.ErrorList
}

func (r *ValidationResults) Append(additionalResults ValidationResults) {
	r.AddErrors(additionalResults.Errors...)
	r.AddWarnings(additionalResults.Warnings...)
}

func (r *ValidationResults) AddErrors(errors ...*field.Error) {
	if len(errors) == 0 {
		return
	}
	r.Errors = append(r.Errors, errors...)
}

func (r *ValidationResults) AddWarnings(warnings ...*field.Error) {
	if len(warnings) == 0 {
		return
	}
	r.Warnings = append(r.Warnings, warnings...)
}
